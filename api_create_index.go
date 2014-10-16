//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the
//  License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an "AS
//  IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
//  express or implied. See the License for the specific language
//  governing permissions and limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/blevesearch/bleve"

	"github.com/gorilla/mux"
)

type CreateIndexHandler struct {
	basePath string
}

func NewCreateIndexHander(basePath string) *CreateIndexHandler {
	return &CreateIndexHandler{
		basePath: basePath,
	}
}

func (h *CreateIndexHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// find the name of the index to create
	indexName := mux.Vars(req)["indexName"]
	if indexName == "" {
		showError(w, req, "index name is required", 400)
		return
	}

	indexMapping := bleve.NewIndexMapping()

	// read the request body
	requestBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		showError(w, req, fmt.Sprintf("error reading request body: %v", err), 400)
		return
	}

	// interpret request body as index mapping
	if len(requestBody) > 0 {
		err := json.Unmarshal(requestBody, &indexMapping)
		if err != nil {
			showError(w, req, fmt.Sprintf("error parsing index mapping: %v", err), 400)
			return
		}
	}

	newIndex, err := bleve.New(h.indexPath(indexName), indexMapping)
	if err != nil {
		showError(w, req, fmt.Sprintf("error creating index: %v", err), 500)
		return
	}

	// make sure there is a bucket with this name
	stream, err := NewTAPFeed(*server, "default", indexName, "")
	if err != nil {
		showError(w, req, fmt.Sprintf("error preparing tap stream: %v", err), 400)
		return
	}

	err = StartRegisteredStream(stream, indexName, newIndex)
	if err != nil {
		showError(w, req, fmt.Sprintf("error starting registered stream: %v", err), 500)
		return
	}

	rv := struct {
		Status string `json:"status"`
	}{
		Status: "ok",
	}
	mustEncode(w, rv)
}

func (h *CreateIndexHandler) indexPath(name string) string {
	return h.basePath + string(os.PathSeparator) + name
}
