package jsoncontract

import "github.com/taeyeong/spec-graph/internal/model"

type EntityResponse struct {
	Entity model.Entity `json:"entity"`
}

type EntityListResponse struct {
	Entities []model.Entity `json:"entities"`
	Count    int            `json:"count"`
}

type RelationResponse struct {
	Relation model.Relation `json:"relation"`
}

type RelationListResponse struct {
	Relations []model.Relation `json:"relations"`
	Count     int              `json:"count"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type DeleteResponse struct {
	Deleted string `json:"deleted"`
}

type InitResponse struct {
	Initialized bool   `json:"initialized"`
	Path        string `json:"path"`
}
