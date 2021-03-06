package main

import (
	"github.com/GeoNet/weft"
	"net/http"
)

type dataType struct {
	typePK int
	Scale  float64 // used to scale the stored metric for display
	Name   string
	Unit   string // display unit after the metric has been multiplied by scale.
}

var dataTypes = map[string]dataType{
	"latency.strong": {
		typePK: 1,
		Scale:  1.0,
		Name:   "latency strong motion data",
		Unit:   "ms",
	},
	"latency.weak": {
		typePK: 2,
		Scale:  1.0,
		Name:   "latency weak motion data",
		Unit:   "ms",
	},
	"latency.gnss.1hz": {
		typePK: 3,
		Scale:  1.0,
		Name:   "latency GNSS 1Hz data",
		Unit:   "ms",
	},
	"latency.tsunami": {
		typePK: 4,
		Scale:  1.0,
		Name:   "latency tsunami data",
		Unit:   "ms",
	},
}

func (d *dataType) load(r *http.Request) *weft.Result {
	var res *weft.Result
	var t dataType
	if t, res = loadDataType(r.URL.Query().Get("typeID")); !res.Ok {
		return res
	}

	// TODO - do we need to copy the values like this?  Revisit.
	d.typePK = t.typePK
	d.Scale = t.Scale
	d.Name = t.Name
	d.Unit = t.Unit
	return &weft.StatusOK
}

func loadDataType(typeID string) (dataType, *weft.Result) {

	if f, ok := dataTypes[typeID]; ok {
		return f, &weft.StatusOK
	}

	return dataType{}, weft.BadRequest("invalid type " + typeID)
}
