package jirarest

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	jira "acli-plus/internal/domain/jira"
	"acli-plus/internal/markdown"
)

// jiraTime parses the timestamp formats the Jira API emits: the platform API
// uses a millisecond stamp with a numeric zone offset, the Agile API uses
// RFC 3339, and date-only fields carry no time at all.
type jiraTime struct{ time.Time }

var timeLayouts = []string{
	"2006-01-02T15:04:05.000-0700",
	"2006-01-02T15:04:05-0700",
	time.RFC3339,
	"2006-01-02",
}

func (t *jiraTime) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		// A null or unexpected shape simply means "no timestamp".
		return nil
	}
	if value == "" {
		return nil
	}
	for _, layout := range timeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return nil
}

// flexID accepts an id that the API may encode as a number or a string.
type flexID string

func (f *flexID) UnmarshalJSON(data []byte) error {
	trimmed := strings.Trim(string(data), `"`)
	if trimmed == "null" {
		trimmed = ""
	}
	*f = flexID(trimmed)
	return nil
}

type userDTO struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Active       bool   `json:"active"`
}

func (d userDTO) toDomain() jira.User {
	return jira.User{
		AccountID:   d.AccountID,
		DisplayName: d.DisplayName,
		Email:       d.EmailAddress,
		Active:      d.Active,
	}
}

func userOrZero(d *userDTO) jira.User {
	if d == nil {
		return jira.User{}
	}
	return d.toDomain()
}

// namedDTO covers the many Jira objects rendered as {"name": "..."}.
type namedDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func nameOrEmpty(d *namedDTO) string {
	if d == nil {
		return ""
	}
	return d.Name
}

func namesOf(items []namedDTO) []string {
	if len(items) == 0 {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

type statusDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category struct {
		Name string `json:"name"`
	} `json:"statusCategory"`
}

type issueDTO struct {
	ID     string         `json:"id"`
	Key    string         `json:"key"`
	Fields issueFieldsDTO `json:"fields"`
}

// issueFieldsDTO models the fields the commands display. Anything not listed
// here still reaches the user through --json, which prints the raw payload.
type issueFieldsDTO struct {
	Summary     string          `json:"summary"`
	Description json.RawMessage `json:"description"`
	Status      *statusDTO      `json:"status"`
	IssueType   *namedDTO       `json:"issuetype"`
	Project     *struct {
		Key string `json:"key"`
	} `json:"project"`
	Priority    *namedDTO  `json:"priority"`
	Resolution  *namedDTO  `json:"resolution"`
	Assignee    *userDTO   `json:"assignee"`
	Reporter    *userDTO   `json:"reporter"`
	Labels      []string   `json:"labels"`
	Components  []namedDTO `json:"components"`
	FixVersions []namedDTO `json:"fixVersions"`
	Parent      *struct {
		Key string `json:"key"`
	} `json:"parent"`
	Created jiraTime `json:"created"`
	Updated jiraTime `json:"updated"`
	DueDate string   `json:"duedate"`
}

// decodeWorkItem turns one raw issue payload into a domain work item, keeping
// the original JSON so --json can show fields this struct does not model.
func decodeWorkItem(raw json.RawMessage) (jira.WorkItem, error) {
	var dto issueDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return jira.WorkItem{}, err
	}

	fields := dto.Fields
	item := jira.WorkItem{
		ID:          dto.ID,
		Key:         dto.Key,
		Summary:     fields.Summary,
		Description: markdown.ADFToText(fields.Description),
		Type:        nameOrEmpty(fields.IssueType),
		Priority:    nameOrEmpty(fields.Priority),
		Resolution:  nameOrEmpty(fields.Resolution),
		Assignee:    userOrZero(fields.Assignee),
		Reporter:    userOrZero(fields.Reporter),
		Labels:      fields.Labels,
		Components:  namesOf(fields.Components),
		FixVersions: namesOf(fields.FixVersions),
		Created:     fields.Created.Time,
		Updated:     fields.Updated.Time,
		DueDate:     fields.DueDate,
		Raw:         raw,
	}
	if fields.Status != nil {
		item.Status = fields.Status.Name
		item.StatusCat = fields.Status.Category.Name
	}
	if fields.Project != nil {
		item.ProjectKey = fields.Project.Key
	}
	if fields.Parent != nil {
		item.ParentKey = fields.Parent.Key
	}
	return item, nil
}

func decodeWorkItems(raws []json.RawMessage) ([]jira.WorkItem, error) {
	items := make([]jira.WorkItem, 0, len(raws))
	for _, raw := range raws {
		item, err := decodeWorkItem(raw)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

type commentDTO struct {
	ID         string          `json:"id"`
	Author     *userDTO        `json:"author"`
	Body       json.RawMessage `json:"body"`
	Created    jiraTime        `json:"created"`
	Updated    jiraTime        `json:"updated"`
	Visibility *struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"visibility"`
}

func (d commentDTO) toDomain() jira.Comment {
	comment := jira.Comment{
		ID:      d.ID,
		Author:  userOrZero(d.Author),
		Body:    markdown.ADFToText(d.Body),
		Created: d.Created.Time,
		Updated: d.Updated.Time,
		Raw:     d.Body,
	}
	if d.Visibility != nil {
		comment.Visibility = jira.CommentVisibility{Type: d.Visibility.Type, Value: d.Visibility.Value}
	}
	return comment
}

type transitionDTO struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	To     *statusDTO `json:"to"`
	Screen bool       `json:"hasScreen"`
}

func (d transitionDTO) toDomain() jira.Transition {
	transition := jira.Transition{ID: d.ID, Name: d.Name, HasScreen: d.Screen}
	if d.To != nil {
		transition.ToName = d.To.Name
	}
	return transition
}

type attachmentDTO struct {
	ID       flexID   `json:"id"`
	Filename string   `json:"filename"`
	MimeType string   `json:"mimeType"`
	Size     int64    `json:"size"`
	Author   *userDTO `json:"author"`
	Created  jiraTime `json:"created"`
}

func (d attachmentDTO) toDomain() jira.Attachment {
	return jira.Attachment{
		ID:       string(d.ID),
		Filename: d.Filename,
		MimeType: d.MimeType,
		Size:     d.Size,
		Author:   userOrZero(d.Author),
		Created:  d.Created.Time,
	}
}

type linkTypeDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

func (d linkTypeDTO) toDomain() jira.LinkType {
	return jira.LinkType{ID: d.ID, Name: d.Name, Inward: d.Inward, Outward: d.Outward}
}

type projectDTO struct {
	ID             flexID   `json:"id"`
	Key            string   `json:"key"`
	Name           string   `json:"name"`
	ProjectTypeKey string   `json:"projectTypeKey"`
	Style          string   `json:"style"`
	Lead           *userDTO `json:"lead"`
	Self           string   `json:"self"`
	Archived       bool     `json:"archived"`
	Deleted        bool     `json:"deleted"`
}

func (d projectDTO) toDomain(host string, raw json.RawMessage) jira.Project {
	project := jira.Project{
		ID:       string(d.ID),
		Key:      d.Key,
		Name:     d.Name,
		TypeKey:  d.ProjectTypeKey,
		Style:    d.Style,
		Lead:     userOrZero(d.Lead),
		Archived: d.Archived,
		Deleted:  d.Deleted,
		Raw:      raw,
	}
	if host != "" && d.Key != "" {
		project.URL = "https://" + host + "/browse/" + d.Key
	}
	return project
}

type fieldDTO struct {
	ID          string   `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Custom      bool     `json:"custom"`
	Searchable  bool     `json:"searchable"`
	ClauseNames []string `json:"clauseNames"`
	Schema      *struct {
		Type   string `json:"type"`
		Items  string `json:"items"`
		Custom string `json:"custom"`
	} `json:"schema"`
}

func (d fieldDTO) toDomain() jira.Field {
	field := jira.Field{
		ID:          d.ID,
		Key:         d.Key,
		Name:        d.Name,
		Custom:      d.Custom,
		Searchable:  d.Searchable,
		ClauseNames: d.ClauseNames,
	}
	if d.Schema != nil {
		field.SchemaType = d.Schema.Type
		field.ItemType = d.Schema.Items
		field.CustomType = d.Schema.Custom
	}
	return field
}

type boardDTO struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location *struct {
		ProjectKey string `json:"projectKey"`
	} `json:"location"`
}

func (d boardDTO) toDomain(host string) jira.Board {
	board := jira.Board{ID: d.ID, Name: d.Name, Type: d.Type}
	if d.Location != nil {
		board.ProjectKey = d.Location.ProjectKey
	}
	if host != "" {
		board.URL = "https://" + host + "/jira/software/projects/" + board.ProjectKey +
			"/boards/" + strconv.Itoa(d.ID)
	}
	return board
}

type sprintDTO struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	State         string   `json:"state"`
	Goal          string   `json:"goal"`
	OriginBoardID int      `json:"originBoardId"`
	StartDate     jiraTime `json:"startDate"`
	EndDate       jiraTime `json:"endDate"`
	CompleteDate  jiraTime `json:"completeDate"`
}

func (d sprintDTO) toDomain() jira.Sprint {
	return jira.Sprint{
		ID:        d.ID,
		BoardID:   d.OriginBoardID,
		Name:      d.Name,
		State:     d.State,
		Goal:      d.Goal,
		Start:     d.StartDate.Time,
		End:       d.EndDate.Time,
		Completed: d.CompleteDate.Time,
	}
}

type filterDTO struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	JQL        string   `json:"jql"`
	Owner      *userDTO `json:"owner"`
	Favourite  bool     `json:"favourite"`
	ViewURL    string   `json:"viewUrl"`
	SharePerms []struct {
		Type    string `json:"type"`
		Project *struct {
			Key string `json:"key"`
		} `json:"project"`
		Group *struct {
			Name string `json:"name"`
		} `json:"group"`
	} `json:"sharePermissions"`
}

func (d filterDTO) toDomain() jira.Filter {
	filter := jira.Filter{
		ID:        d.ID,
		Name:      d.Name,
		JQL:       d.JQL,
		Owner:     userOrZero(d.Owner),
		Favourite: d.Favourite,
		URL:       d.ViewURL,
	}
	for _, perm := range d.SharePerms {
		switch {
		case perm.Project != nil && perm.Project.Key != "":
			filter.SharedWith = append(filter.SharedWith, "project:"+perm.Project.Key)
		case perm.Group != nil && perm.Group.Name != "":
			filter.SharedWith = append(filter.SharedWith, "group:"+perm.Group.Name)
		default:
			filter.SharedWith = append(filter.SharedWith, perm.Type)
		}
	}
	return filter
}

type dashboardDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Owner       *userDTO `json:"owner"`
	View        string   `json:"view"`
	IsFavourite bool     `json:"isFavourite"`
	Popularity  int      `json:"popularity"`
}

func (d dashboardDTO) toDomain() jira.Dashboard {
	return jira.Dashboard{
		ID:         d.ID,
		Name:       d.Name,
		Owner:      userOrZero(d.Owner),
		URL:        d.View,
		Favourite:  d.IsFavourite,
		Popularity: d.Popularity,
	}
}
