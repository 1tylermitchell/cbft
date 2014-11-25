//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/couchbaselabs/clog"
)

func docIDLookup(req *http.Request) string {
	return muxVariableLookup(req, "docID")
}

func indexNameLookup(req *http.Request) string {
	return muxVariableLookup(req, "indexName")
}

// ------------------------------------------------------------------

type ListIndexHandler struct {
	mgr *Manager
}

func NewListIndexHandler(mgr *Manager) *ListIndexHandler {
	return &ListIndexHandler{mgr: mgr}
}

func (h *ListIndexHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	indexDefs, _, err := h.mgr.GetIndexDefs(false)
	if err != nil {
		showError(w, req, "could not retrieve index defs", 500)
		return
	}

	rv := struct {
		Status    string     `json:"status"`
		IndexDefs *IndexDefs `json:"indexDefs"`
	}{
		Status:    "ok",
		IndexDefs: indexDefs,
	}
	mustEncode(w, rv)
}

// ---------------------------------------------------

type GetIndexHandler struct {
	mgr *Manager
}

func NewGetIndexHandler(mgr *Manager) *GetIndexHandler {
	return &GetIndexHandler{mgr: mgr}
}

func (h *GetIndexHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	indexName := indexNameLookup(req)
	if indexName == "" {
		showError(w, req, "index name is required", 400)
		return
	}

	_, indexDefsByName, err := h.mgr.GetIndexDefs(false)
	if err != nil {
		showError(w, req, "could not retrieve index defs", 500)
		return
	}

	indexDef, exists := indexDefsByName[indexName]
	if !exists || indexDef == nil {
		showError(w, req, "not an index", 400)
		return
	}

	indexUUID := req.FormValue("indexUUID")
	if indexUUID != "" && indexUUID != indexDef.UUID {
		showError(w, req, "wrong index UUID", 400)
		return
	}

	m := map[string]interface{}{}
	if indexDef.Schema != "" {
		if err := json.Unmarshal([]byte(indexDef.Schema), &m); err != nil {
			showError(w, req, "could not unmarshal mapping", 500)
			return
		}
	}

	rv := struct {
		Status       string                 `json:"status"`
		IndexDef     *IndexDef              `json:"indexDef"`
		IndexMapping map[string]interface{} `json:"indexMapping"`
	}{
		Status:       "ok",
		IndexDef:     indexDef,
		IndexMapping: m,
	}
	mustEncode(w, rv)
}

// ---------------------------------------------------

type CountHandler struct {
	mgr *Manager
}

func NewCountHandler(mgr *Manager) *CountHandler {
	return &CountHandler{mgr: mgr}
}

func (h *CountHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	indexName := indexNameLookup(req)
	if indexName == "" {
		showError(w, req, "index name is required", 400)
		return
	}

	indexUUID := req.FormValue("indexUUID")

	indexDefs, _, err := CfgGetIndexDefs(h.mgr.cfg)
	if err != nil {
		showError(w, req, fmt.Sprintf("rest.Count,"+
			" could not get indexDefs, indexName: %s, err: %v",
			indexName, err), 400)
		return
	}
	indexDef := indexDefs.IndexDefs[indexName]
	if indexDef == nil {
		showError(w, req, fmt.Sprintf("rest.Count,"+
			" no indexDef, indexName: %s", indexName), 400)
		return
	}
	pindexImplType := pindexImplTypes[indexDef.Type]
	if pindexImplType == nil ||
		pindexImplType.Count == nil {
		showError(w, req, fmt.Sprintf("rest.Count,"+
			" no pindexImplType, indexName: %s, indexDef.Type: %s",
			indexName, indexDef.Type), 400)
		return
	}

	count, err := pindexImplType.Count(h.mgr, indexName, indexUUID)
	if err != nil {
		showError(w, req, fmt.Sprintf("rest.Count,"+
			" indexName: %s1, err: %v", indexName, err), 500)
		return
	}

	rv := struct {
		Status string `json:"status"`
		Count  uint64 `json:"count"`
	}{
		Status: "ok",
		Count:  count,
	}
	mustEncode(w, rv)
}

// ---------------------------------------------------

type SearchHandler struct {
	mgr *Manager
}

func NewSearchHandler(mgr *Manager) *SearchHandler {
	return &SearchHandler{mgr: mgr}
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	indexName := indexNameLookup(req)
	if indexName == "" {
		showError(w, req, "index name is required", 400)
		return
	}

	indexUUID := req.FormValue("indexUUID")

	indexDefs, _, err := CfgGetIndexDefs(h.mgr.cfg)
	if err != nil {
		showError(w, req, fmt.Sprintf("rest.Search,"+
			" could not get indexDefs, indexName: %s, err: %v",
			indexName, err), 400)
		return
	}
	indexDef := indexDefs.IndexDefs[indexName]
	if indexDef == nil {
		showError(w, req, fmt.Sprintf("rest.Search,"+
			" no indexDef, indexName: %s", indexName), 400)
		return
	}
	pindexImplType := pindexImplTypes[indexDef.Type]
	if pindexImplType == nil ||
		pindexImplType.Query == nil {
		showError(w, req, fmt.Sprintf("rest.Search,"+
			" no pindexImplType, indexName: %s, indexDef.Type: %s",
			indexName, indexDef.Type), 400)
		return
	}

	requestBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		showError(w, req, fmt.Sprintf("rest.Search,"+
			" could not read request body, indexName: %s, indexDef.Type: %s",
			indexName, indexDef.Type), 400)
		return
	}
	log.Printf("rest.Search indexName: %s, indexDef.Type: %s,"+
		" requestBody: %s", indexName, indexDef.Type, requestBody)

	err = pindexImplType.Query(h.mgr, indexName, indexUUID, requestBody, w)
	if err != nil {
		showError(w, req, fmt.Sprintf("rest.Search,"+
			" indexName: %s, indexDef.Type: %s, requestBody: %s, err: %v",
			indexName, indexDef.Type, requestBody, err), 400)
		return
	}
}