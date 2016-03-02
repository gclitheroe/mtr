package main

import (
	"bytes"
	"github.com/lib/pq"
	"net/http"
	"strconv"
)

type fieldThreshold struct {
	lower, upper int
}

func (f *fieldThreshold) save(r *http.Request) *result {
	if res := checkQuery(r, []string{"deviceID", "typeID", "lower", "upper"}, []string{}); !res.ok {
		return res
	}

	var err error

	if f.lower, err = strconv.Atoi(r.URL.Query().Get("lower")); err != nil {
		return badRequest("invalid lower")
	}

	if f.upper, err = strconv.Atoi(r.URL.Query().Get("upper")); err != nil {
		return badRequest("invalid upper")
	}

	var fm fieldMetric

	if res := fm.loadPK(r); !res.ok {
		return res
	}

	if _, err = db.Exec(`INSERT INTO field.threshold(devicePK, typePK, lower, upper) 
		VALUES ($1,$2,$3,$4)`,
		fm.devicePK, fm.typePK, f.lower, f.upper); err != nil {
		if err, ok := err.(*pq.Error); ok && err.Code == `23505` {
			// ignore unique constraint errors
		} else {
			return internalServerError(err)
		}
	}

	if _, err = db.Exec(`UPDATE field.threshold SET lower=$3, upper=$4 
		WHERE 
		devicePK = $1 
		AND
		typePK = $2`,
		fm.devicePK, fm.typePK, f.lower, f.upper); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func (f *fieldThreshold) delete(r *http.Request) *result {
	if res := checkQuery(r, []string{"deviceID", "typeID"}, []string{}); !res.ok {
		return res
	}

	var err error

	var fm fieldMetric

	if res := fm.loadPK(r); !res.ok {
		return res
	}

	if _, err = db.Exec(`DELETE FROM field.threshold 
		WHERE devicePK = $1
		AND typePK = $2 `,
		fm.devicePK, fm.typePK); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func (f *fieldThreshold) jsonV1(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	if res := checkQuery(r, []string{}, []string{}); !res.ok {
		return res
	}

	var s string

	if err := dbR.QueryRow(`SELECT COALESCE(array_to_json(array_agg(row_to_json(l))), '[]')
		FROM (SELECT deviceID as "DeviceID", typeID as "TypeID", 
		lower as "Lower", upper AS "Upper" 
		FROM 
		field.threshold JOIN field.device USING (devicepk) 
		JOIN field.type USING (typepk)) l`).Scan(&s); err != nil {
		return internalServerError(err)
	}

	b.WriteString(s)

	h.Set("Content-Type", "application/json;version=1")

	return &statusOK
}
