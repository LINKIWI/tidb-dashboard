// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package endpoint

import (
	"net/http"
	"net/url"
	"regexp"

	"github.com/joomcode/errorx"

	"github.com/pingcap/tidb-dashboard/pkg/apiserver/model"
)

var (
	ErrNS                   = errorx.NewNamespace("error.api.debugapi.endpoint")
	ErrMissingRequiredParam = ErrNS.NewType("missing_require_parameter")
	ErrInvalidParam         = ErrNS.NewType("invalid_parameter")
)

type APIModel struct {
	ID          string         `json:"id"`
	Component   model.NodeKind `json:"component"`
	Path        string         `json:"path"`
	Method      Method         `json:"method"`
	PathParams  []APIParam     `json:"path_params"`  // e.g. /stats/dump/{db}/{table} -> db, table
	QueryParams []APIParam     `json:"query_params"` // e.g. /debug/pprof?seconds=1 -> seconds
}

type APIParam struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	// represents what param is
	Model                APIParamModel    `json:"model" swaggertype:"object,string"`
	PreModelTransformer  ModelTransformer `json:"-"`
	PostModelTransformer ModelTransformer `json:"-"`
}

type APIParamModel interface {
	Transform(ctx *Context) error
	PreTransform(ctx *Context) error
}

// ModelTransformer can transform the incoming param's value in special scenarios
// Also, now are used as validation function
type ModelTransformer func(ctx *Context) error

type Request struct {
	Method Method
	Host   string
	Port   int
	Path   string
	Query  string
}

type Method string

const (
	MethodGet Method = http.MethodGet
)

func (m *APIModel) NewRequest(host string, port int, data map[string]string) (*Request, error) {
	req := &Request{
		Method: m.Method,
		Host:   host,
		Port:   port,
	}

	pathValues, err := transformValues(m.PathParams, data, true)
	if err != nil {
		return nil, err
	}
	path, err := populatePath(m.Path, pathValues)
	if err != nil {
		return nil, err
	}
	req.Path = path

	queryValues, err := transformValues(m.QueryParams, data, false)
	if err != nil {
		return nil, err
	}
	query, err := encodeQuery(m.QueryParams, queryValues)
	if err != nil {
		return nil, err
	}
	req.Query = query

	return req, nil
}

var paramRegexp *regexp.Regexp = regexp.MustCompile(`\{(\w+)\}`)

func populatePath(path string, values Values) (string, error) {
	var returnErr error
	replacedPath := paramRegexp.ReplaceAllStringFunc(path, func(s string) string {
		if returnErr != nil {
			return s
		}
		key := paramRegexp.ReplaceAllString(s, "${1}")
		val := values.Get(key)
		return val
	})
	return replacedPath, returnErr
}

func encodeQuery(queryParams []APIParam, values Values) (string, error) {
	query := url.Values{}
	for _, q := range queryParams {
		vals := values[q.Name]
		if len(vals) == 0 {
			continue
		}
		for _, val := range vals {
			query.Add(q.Name, val)
		}
	}
	return query.Encode(), nil
}

// Transform incoming param's value by transformer at endpoint / model definition
func transformValues(params []APIParam, values map[string]string, forceRequired bool) (Values, error) {
	vs := Values{}
	for _, p := range params {
		if v, ok := values[p.Name]; ok {
			vs.Set(p.Name, v)
		}
	}

	for _, p := range params {
		ctx := &Context{
			paramValues: vs,
			paramName:   p.Name,
		}

		// PreModelTransformer can be used to set default value
		err := transform(ctx, p.PreModelTransformer)
		if err != nil {
			return nil, ErrInvalidParam.Wrap(err, "param: %s", p.Name)
		}
		err = transform(ctx, p.Model.PreTransform)
		if err != nil {
			return nil, ErrInvalidParam.Wrap(err, "param: %s", p.Name)
		}
		if ctx.Value() == "" {
			if forceRequired || p.Required {
				return nil, ErrMissingRequiredParam.New("missing required param: %s", p.Name)
			}
			// There's no value from the client or default value generate from the pre-transformer,
			// so we can skip the model transformer and the post-transformer
			continue
		}

		err = transform(ctx, p.Model.Transform)
		if err != nil {
			return nil, ErrInvalidParam.Wrap(err, "param: %s", p.Name)
		}

		err = transform(ctx, p.PostModelTransformer)
		if err != nil {
			return nil, ErrInvalidParam.Wrap(err, "param: %s", p.Name)
		}
	}

	return vs, nil
}

func transform(ctx *Context, transformer ModelTransformer) error {
	if transformer == nil {
		return nil
	}
	return transformer(ctx)
}
