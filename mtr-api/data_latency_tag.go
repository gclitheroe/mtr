package main

import (
	"bytes"
	"database/sql"
	"github.com/GeoNet/mtr/mtrpb"
	"github.com/GeoNet/weft"
	"github.com/golang/protobuf/proto"
	"github.com/lib/pq"
	"net/http"
)

type dataLatencyTag struct {
	tag
	dataSite
	dataType
}

func (a *dataLatencyTag) read() *weft.Result {
	if res := a.dataType.read(); !res.Ok {
		return res
	}

	if res := a.dataSite.read(); !res.Ok {
		return res
	}

	if res := a.tag.read(); !res.Ok {
		return res
	}

	return &weft.StatusOK
}

func (f *dataLatencyTag) save(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID", "tag"}, []string{}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	f.dataType.id = v.Get("typeID")
	f.dataSite.id = v.Get("siteID")
	f.tag.id = v.Get("tag")

	if res := f.read(); !res.Ok {
		return res
	}

	if _, err := db.Exec(`INSERT INTO data.latency_tag(sitePK, typePK, tagPK)
			VALUES($1, $2, $3)`,
		f.dataSite.pk, f.dataType.pk, f.tag.pk); err != nil {
		if err, ok := err.(*pq.Error); ok && err.Code == errorUniqueViolation {
			// ignore unique constraint errors
		} else {
			return weft.InternalServerError(err)
		}
	}

	return &weft.StatusOK
}

func (f *dataLatencyTag) delete(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID", "tag"}, []string{}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	f.dataType.id = v.Get("typeID")
	f.dataSite.id = v.Get("siteID")
	f.tag.id = v.Get("tag")

	if res := f.read(); !res.Ok {
		return res
	}

	if _, err := db.Exec(`DELETE FROM data.latency_tag
			WHERE sitePK = $1
			AND typePK = $2
			AND tagPK = $3`, f.dataSite.pk, f.dataType.pk, f.tag.pk); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

func (t *dataLatencyTag) all(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{}, []string{}); !res.Ok {
		return res
	}

	var err error
	var rows *sql.Rows

	if rows, err = dbR.Query(`SELECT siteID, tag, typeID from data.latency_tag
				JOIN mtr.tag USING (tagpk)
				JOIN data.site USING (sitepk)
				JOIN data.type USING (typepk)
				ORDER BY tag ASC`); err != nil {
		return weft.InternalServerError(err)
	}
	defer rows.Close()

	var ts mtrpb.DataLatencyTagResult

	for rows.Next() {
		var t mtrpb.DataLatencyTag

		if err = rows.Scan(&t.SiteID, &t.Tag, &t.TypeID); err != nil {
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
