// Package confluencerest is the Confluence Cloud REST (API v2) adapter. It
// implements the confluence.Gateway port and maps API payloads to domain types.
package confluencerest

import (
	"bytes"
	"encoding/json"
	"strings"

	confluence "acli-plus/internal/domain/confluence"
)

// flexID decodes an id that the API may serialize as either a JSON string or a
// number, always yielding a string.
type flexID string

func (f *flexID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		*f = ""
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		*f = flexID(s)
		return nil
	}
	*f = flexID(strings.Trim(string(trimmed), `"`))
	return nil
}

type bodyDTO struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

type versionDTO struct {
	Number  int    `json:"number"`
	Message string `json:"message"`
}

type pageDTO struct {
	ID       flexID     `json:"id"`
	Title    string     `json:"title"`
	SpaceID  flexID     `json:"spaceId"`
	ParentID flexID     `json:"parentId"`
	Version  versionDTO `json:"version"`
}

func (p pageDTO) toDomain() confluence.Page {
	return confluence.Page{
		ID:       string(p.ID),
		Title:    p.Title,
		SpaceID:  string(p.SpaceID),
		ParentID: string(p.ParentID),
		Version: confluence.Version{
			Number:  p.Version.Number,
			Message: p.Version.Message,
		},
	}
}

type createPageRequest struct {
	SpaceID  string  `json:"spaceId"`
	Status   string  `json:"status"`
	Title    string  `json:"title"`
	ParentID string  `json:"parentId,omitempty"`
	Body     bodyDTO `json:"body"`
}

type versionRequest struct {
	Number  int    `json:"number"`
	Message string `json:"message,omitempty"`
}

type updatePageRequest struct {
	ID      string         `json:"id"`
	Status  string         `json:"status"`
	Title   string         `json:"title"`
	Body    bodyDTO        `json:"body"`
	Version versionRequest `json:"version"`
}
