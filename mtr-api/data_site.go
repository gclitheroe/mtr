package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/GeoNet/mtr/mtrpb"
	"github.com/GeoNet/weft"
	"github.com/golang/protobuf/proto"
	"github.com/lib/pq"
	"net/http"
	"strconv"
)

type dataSite struct {
	pk                  int
	id                  string
	longitude, latitude float64
}

// createsUpdate creates or updates the dataSite in the db.
func (a *dataSite) createUpdate() *weft.Result {
	if a.id == "" {
		return weft.InternalServerError(fmt.Errorf("empty dataSite.id"))
	}

	// TODO convert to upsert with pg 9.5
	if _, err := db.Exec(`INSERT INTO data.site(siteID, latitude, longitude) VALUES($1, $2, $3)`,
		a.id, a.latitude, a.longitude); err != nil {
		if err, ok := err.(*pq.Error); ok && err.Code == errorUniqueViolation {
			if _, err := db.Exec(`UPDATE data.site SET latitude=$2, longitude=$3 where siteID=$1`,
				a.id, a.latitude, a.longitude); err != nil {
				return weft.InternalServerError(err)
			}
		} else {
			return weft.InternalServerError(err)
		}
	}

	return &weft.StatusOK
}

// read sets pk to the database primary key.
func (a *dataSite) read() *weft.Result {
	if a.id == "" {
		return weft.InternalServerError(fmt.Errorf("empty dataSite.id"))
	}

	if err := dbR.QueryRow(`SELECT sitePK FROM data.site where siteID = $1`,
		a.id).Scan(&a.pk); err != nil {
		if err == sql.ErrNoRows {
			return weft.BadRequest("unknown siteID")
		}
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

func (a *dataSite) del() *weft.Result {
	if a.id == "" {
		return weft.InternalServerError(fmt.Errorf("empty dataSite.id"))
	}

	if _, err := db.Exec(`DELETE FROM data.site where siteID = $1`, a.id); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

func (a *dataSite) put(r *http.Request) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "latitude", "longitude"}, []string{}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	a.id = v.Get("siteID")

	var err error

	if a.latitude, err = strconv.ParseFloat(v.Get("latitude"), 64); err != nil {
		return weft.BadRequest("latitude invalid")
	}

	if a.longitude, err = strconv.ParseFloat(v.Get("longitude"), 64); err != nil {
		return weft.BadRequest("longitude invalid")
	}

	return a.createUpdate()
}

func (a *dataSite) delete(r *http.Request) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID"}, []string{}); !res.Ok {
		return res
	}

	a.id = r.URL.Query().Get("siteID")

	return a.del()
}

func (a *dataSite) proto(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{}, []string{}); !res.Ok {
		return res
	}

	var err error
	var rows *sql.Rows

	if rows, err = dbR.Query(`SELECT siteID, latitude, longitude FROM data.site`); err != nil {
		return weft.InternalServerError(err)
	}

	var ts mtrpb.DataSiteResult

	for rows.Next() {
		var t mtrpb.DataSite

		if err = rows.Scan(&t.SiteID, &t.Latitude, &t.Longitude); err != nil {
			return weft.InternalServerError(err)
		}

		ts.Result = append(ts.Result, &t)
	}

	var by []byte
	if by, err = proto.Marshal(&ts); err != nil {
		return weft.InternalServerError(err)
	}

	b.Write(by)

	h.Set("Content-Type", "application/x-protobuf")

	return &weft.StatusOK
}
