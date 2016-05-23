package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/GeoNet/mtr/ts"
	"github.com/GeoNet/weft"
	"github.com/lib/pq"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type dataLatency struct {
	dataSite
	dataType
	pk                            *weft.Result // for tracking pkLoad()
	t                             time.Time
	mean, min, max, fifty, ninety int
}

func (a *dataLatency) read() *weft.Result {
	if res := a.dataType.read(); !res.Ok {
		return res
	}

	if res := a.dataSite.read(); !res.Ok {
		return res
	}

	return &weft.StatusOK
}

func (a *dataLatency) create() *weft.Result {
	if res := a.read(); !res.Ok {
		return res
	}

	if _, err := db.Exec(`INSERT INTO data.latency(sitePK, typePK, rate_limit, time, mean, min, max, fifty, ninety) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.dataSite.pk, a.dataType.pk, a.t.Truncate(time.Minute).Unix(),
		a.t, int32(a.mean), int32(a.min), int32(a.max), int32(a.fifty), int32(a.ninety)); err != nil {
		if err, ok := err.(*pq.Error); ok && err.Code == errorUniqueViolation {
			return &statusTooManyRequests
		} else {
			return weft.InternalServerError(err)
		}
	}

	return &weft.StatusOK
}

/*
loadThreshold loads thresholds for the data latency.  Assumes d.loadPK has been called first.
*/
func (d *dataLatency) threshold() (lower, upper int, res *weft.Result) {
	res = &weft.StatusOK

	if err := dbR.QueryRow(`SELECT lower,upper FROM data.latency_threshold
		WHERE sitePK = $1 AND typePK = $2`,
		d.dataSite.pk, d.dataType.pk).Scan(&lower, &upper); err != nil && err != sql.ErrNoRows {
		res = weft.InternalServerError(err)
	}

	return
}

func (d *dataLatency) save(r *http.Request) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID", "time", "mean"}, []string{"min", "max", "fifty", "ninety"}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	var err error

	d.dataType.id = v.Get("typeID")
	d.dataSite.id = v.Get("siteID")

	if d.mean, err = strconv.Atoi(v.Get("mean")); err != nil {
		return weft.BadRequest("invalid value for mean")
	}

	if v.Get("min") != "" {
		if d.min, err = strconv.Atoi(v.Get("min")); err != nil {
			return weft.BadRequest("invalid value for min")
		}
	}

	if v.Get("max") != "" {
		if d.max, err = strconv.Atoi(v.Get("max")); err != nil {
			return weft.BadRequest("invalid value for max")
		}
	}

	if v.Get("fifty") != "" {
		if d.fifty, err = strconv.Atoi(v.Get("fifty")); err != nil {
			return weft.BadRequest("invalid value for fifty")
		}
	}

	if v.Get("ninety") != "" {
		if d.ninety, err = strconv.Atoi(v.Get("ninety")); err != nil {
			return weft.BadRequest("invalid value for ninety")
		}
	}

	if d.t, err = time.Parse(time.RFC3339, v.Get("time")); err != nil {
		return weft.BadRequest("invalid time")
	}

	return d.create()
}

func (d *dataLatency) delete(r *http.Request) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID"}, []string{}); !res.Ok {
		return res
	}

	v := r.URL.Query()
	d.dataSite.id = v.Get("siteID")
	d.dataType.id = v.Get("typeID")

	if res := d.read(); !res.Ok {
		return res
	}

	var txn *sql.Tx
	var err error

	if txn, err = db.Begin(); err != nil {
		return weft.InternalServerError(err)
	}

	if _, err = txn.Exec(`DELETE FROM data.latency WHERE sitePK = $1 AND typePK = $2`,
		d.dataSite.pk, d.dataType.pk); err != nil {
		txn.Rollback()
		return weft.InternalServerError(err)
	}

	if _, err = txn.Exec(`DELETE FROM data.latency_threshold WHERE sitePK = $1 AND typePK = $2`,
		d.dataSite.pk, d.dataType.pk); err != nil {
		txn.Rollback()
		return weft.InternalServerError(err)
	}

	if _, err = txn.Exec(`DELETE FROM data.latency_tag WHERE sitePK = $1 AND typePK = $2`,
		d.dataSite.pk, d.dataType.pk); err != nil {
		txn.Rollback()
		return weft.InternalServerError(err)
	}

	if err = txn.Commit(); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

func (d *dataLatency) svg(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID"}, []string{"plot", "resolution", "yrange"}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	d.dataSite.id = v.Get("siteID")
	d.dataType.id = v.Get("typeID")

	if res := d.read(); !res.Ok {
		return res
	}

	switch r.URL.Query().Get("plot") {
	case "", "line":
		resolution := v.Get("resolution")
		if resolution == "" {
			resolution = "minute"
		}
		if res := d.plot(resolution, b); !res.Ok {
			return res
		}
	default:
		if res := d.spark(b); !res.Ok {
			return res
		}
	}

	h.Set("Content-Type", "image/svg+xml")

	return &weft.StatusOK
}

/*
plot draws an svg plot to b.  Assumes f.loadPK has been called first.
*/
func (d *dataLatency) plot(resolution string, b *bytes.Buffer) *weft.Result {
	var p ts.Plot

	p.SetUnit(d.dataType.Unit)

	var lower, upper int
	var res *weft.Result

	if lower, upper, res = d.threshold(); !res.Ok {
		return res
	}

	if !(lower == 0 && upper == 0) {
		p.SetThreshold(float64(lower)*d.dataType.Scale, float64(upper)*d.dataType.Scale)
	}

	var tags []string

	if tags, res = d.tags(); !res.Ok {
		return res
	}

	p.SetSubTitle("Tags: " + strings.Join(tags, ","))

	p.SetTitle(fmt.Sprintf("Site: %s - %s", d.dataSite.id, strings.Title(d.dataType.Name)))

	var err error
	var rows *sql.Rows

	// TODO - loading avg(mean) at each resolution.  Need to add max(fifty) and max(ninety) when there are some values.

	switch resolution {
	case "minute":
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-12), time.Now().UTC())
		p.SetXLabel("12 hours")

		rows, err = dbR.Query(`SELECT date_trunc('`+resolution+`',time) as t, avg(mean) FROM data.latency WHERE
		sitePK = $1 AND typePK = $2
		AND time > now() - interval '12 hours'
		GROUP BY date_trunc('`+resolution+`',time)
		ORDER BY t ASC`,
			d.dataSite.pk, d.dataType.pk)
	case "five_minutes":
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-24*2), time.Now().UTC())
		p.SetXLabel("48 hours")

		rows, err = dbR.Query(`SELECT date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t,
		 avg(mean) FROM data.latency WHERE
		sitePK = $1 AND typePK = $2
		AND time > now() - interval '2 days'
		GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min'
		ORDER BY t ASC`,
			d.dataSite.pk, d.dataType.pk)
	case "hour":
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-24*28), time.Now().UTC())
		p.SetXLabel("4 weeks")

		rows, err = dbR.Query(`SELECT date_trunc('`+resolution+`',time) as t, avg(mean) FROM data.latency WHERE
		sitePK = $1 AND typePK = $2
		AND time > now() - interval '28 days'
		GROUP BY date_trunc('`+resolution+`',time)
		ORDER BY t ASC`,
			d.dataSite.pk, d.dataType.pk)
	default:
		return weft.BadRequest("invalid resolution")
	}
	if err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	var t time.Time
	var avg float64
	var pts []ts.Point

	for rows.Next() {
		if err = rows.Scan(&t, &avg); err != nil {
			return weft.InternalServerError(err)
		}
		pts = append(pts, ts.Point{DateTime: t, Value: avg * d.dataType.Scale})
	}
	rows.Close()

	// Add the latest value to the plot - this may be different to the average at minute or hour resolution.
	t = time.Time{}
	var value int32
	if err = dbR.QueryRow(`SELECT time, mean FROM data.latency WHERE
			sitePK = $1 AND typePK = $2
			ORDER BY time DESC
			LIMIT 1`,
		d.dataSite.pk, d.dataType.pk).Scan(&t, &value); err != nil {
		return weft.InternalServerError(err)
	}

	pts = append(pts, ts.Point{DateTime: t, Value: float64(value) * d.dataType.Scale})
	p.SetLatest(ts.Point{DateTime: t, Value: float64(value) * d.dataType.Scale}, "deepskyblue")

	p.AddSeries(ts.Series{Colour: "deepskyblue", Points: pts})

	if err = ts.Line.Draw(p, b); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

/*
spark draws an svg spark line to b.  Assumes f.loadPK has been called first.
*/
func (d *dataLatency) spark(b *bytes.Buffer) *weft.Result {
	var p ts.Plot

	p.SetXAxis(time.Now().UTC().Add(time.Hour*-12), time.Now().UTC())

	var err error
	var rows *sql.Rows

	if rows, err = dbR.Query(`SELECT date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t,
		 avg(mean) FROM data.latency WHERE
		sitePK = $1 AND typePK = $2
		AND time > now() - interval '12 hours'
		GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min'
		ORDER BY t ASC`,
		d.dataSite.pk, d.dataType.pk); err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	var t time.Time
	var avg float64
	var pts []ts.Point

	for rows.Next() {
		if err = rows.Scan(&t, &avg); err != nil {
			return weft.InternalServerError(err)
		}
		pts = append(pts, ts.Point{DateTime: t, Value: avg * d.dataType.Scale})
	}
	rows.Close()

	p.AddSeries(ts.Series{Colour: "deepskyblue", Points: pts})

	if err = ts.SparkLine.Draw(p, b); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

// tags returns tags for f.  Assumes loadPK has been called.
func (f *dataLatency) tags() (t []string, res *weft.Result) {
	var rows *sql.Rows
	var err error

	if rows, err = dbR.Query(`SELECT tag FROM data.latency_tag JOIN mtr.tag USING (tagpk) WHERE
		sitePK = $1 AND typePK = $2
		ORDER BY tag asc`,
		f.dataSite.pk, f.dataType.pk); err != nil {
		res = weft.InternalServerError(err)
		return
	}

	defer rows.Close()

	var s string

	for rows.Next() {
		if err = rows.Scan(&s); err != nil {
			res = weft.InternalServerError(err)
			return
		}
		t = append(t, s)
	}

	res = &weft.StatusOK
	return
}
