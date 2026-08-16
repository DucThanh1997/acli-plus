package app

import (
	"context"
	"strings"
	"testing"

	confluence "acli-plus/internal/domain/confluence"
)

// fakeGateway is a controllable confluence.Gateway for use-case tests.
type fakeGateway struct {
	getPageFn func(id string) (confluence.Page, error)
	findFn    func(spaceID, title string) (confluence.Page, bool, error)
	spaceFn   func(key string) (string, error)

	created []confluence.NewPageInput
	updated []confluence.UpdatePageInput
	deleted []string
}

func (f *fakeGateway) VerifyAuth(context.Context) error { return nil }

func (f *fakeGateway) ResolveSpaceID(_ context.Context, key string) (string, error) {
	if f.spaceFn != nil {
		return f.spaceFn(key)
	}
	return "100", nil
}

func (f *fakeGateway) GetPage(_ context.Context, id string) (confluence.Page, error) {
	if f.getPageFn != nil {
		return f.getPageFn(id)
	}
	return confluence.Page{}, confluence.ErrPageNotFound
}

func (f *fakeGateway) FindPageByTitle(_ context.Context, spaceID, title string) (confluence.Page, bool, error) {
	if f.findFn != nil {
		return f.findFn(spaceID, title)
	}
	return confluence.Page{}, false, nil
}

func (f *fakeGateway) CreatePage(_ context.Context, in confluence.NewPageInput) (confluence.Page, error) {
	f.created = append(f.created, in)
	return confluence.Page{ID: "new-id", Title: in.Title, SpaceID: in.SpaceID, ParentID: in.ParentID, Version: confluence.Version{Number: 1}}, nil
}

func (f *fakeGateway) UpdatePage(_ context.Context, in confluence.UpdatePageInput) (confluence.Page, error) {
	f.updated = append(f.updated, in)
	return confluence.Page{ID: in.ID, Title: in.Title, Version: confluence.Version{Number: in.NextVersion, Message: in.Message}}, nil
}

func (f *fakeGateway) DeletePage(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func alwaysYes(string) (bool, error) { return true, nil }
func alwaysNo(string) (bool, error)  { return false, nil }

func content() Content { return Content{Title: "Doc", Body: "<p>hi</p>"} }

func TestCreateNewPageUnderSpace(t *testing.T) {
	gw := &fakeGateway{}
	svc := NewPageService(gw)

	res, err := svc.Create(context.Background(), CreateInput{
		Content: content(),
		Parent:  confluence.PageRef{SpaceKey: "DEV"},
		Opts:    WriteOptions{Confirm: alwaysNo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreate {
		t.Fatalf("action = %s, want create", res.Action)
	}
	if len(gw.created) != 1 || gw.created[0].SpaceID != "100" {
		t.Fatalf("expected one create in space 100, got %+v", gw.created)
	}
}

func TestCreateCollapsesToUpdateWhenTitleExists(t *testing.T) {
	gw := &fakeGateway{
		findFn: func(spaceID, title string) (confluence.Page, bool, error) {
			return confluence.Page{ID: "p1", SpaceID: "100"}, true, nil
		},
		getPageFn: func(id string) (confluence.Page, error) {
			return confluence.Page{ID: "p1", Title: "Old", SpaceID: "100", Version: confluence.Version{Number: 2, Message: versionStamp}}, nil
		},
	}
	svc := NewPageService(gw)

	res, err := svc.Create(context.Background(), CreateInput{
		Content: content(),
		Parent:  confluence.PageRef{SpaceKey: "DEV"},
		Opts:    WriteOptions{Confirm: alwaysNo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionUpdate {
		t.Fatalf("action = %s, want update", res.Action)
	}
	if len(gw.created) != 0 {
		t.Fatalf("should not have created a duplicate, got %+v", gw.created)
	}
	if len(gw.updated) != 1 || gw.updated[0].NextVersion != 3 {
		t.Fatalf("expected update to version 3, got %+v", gw.updated)
	}
}

func TestUpdateStampedPageSkipsPrompt(t *testing.T) {
	confirmCalled := false
	gw := &fakeGateway{
		getPageFn: func(id string) (confluence.Page, error) {
			return confluence.Page{ID: "p1", Title: "T", Version: confluence.Version{Number: 4, Message: versionStamp}}, nil
		},
	}
	svc := NewPageService(gw)

	res, err := svc.Update(context.Background(), UpdateInput{
		Content: content(),
		Target:  confluence.PageRef{Host: "h", SpaceKey: "DEV", PageID: "p1"},
		Opts: WriteOptions{Confirm: func(string) (bool, error) {
			confirmCalled = true
			return true, nil
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmCalled {
		t.Error("should not prompt when the latest version was written by acli-plus")
	}
	if res.Action != ActionUpdate || gw.updated[0].NextVersion != 5 {
		t.Fatalf("expected update to version 5, got %+v", gw.updated)
	}
}

func TestUpdateExternallyModifiedPromptsAndAbortsOnNo(t *testing.T) {
	gw := &fakeGateway{
		getPageFn: func(id string) (confluence.Page, error) {
			return confluence.Page{ID: "p1", Title: "T", Version: confluence.Version{Number: 5, Message: ""}}, nil
		},
	}
	svc := NewPageService(gw)

	res, err := svc.Update(context.Background(), UpdateInput{
		Content: content(),
		Target:  confluence.PageRef{PageID: "p1", SpaceKey: "DEV"},
		Opts:    WriteOptions{Confirm: alwaysNo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionAborted {
		t.Fatalf("action = %s, want aborted", res.Action)
	}
	if len(gw.updated) != 0 {
		t.Fatalf("should not have written, got %+v", gw.updated)
	}
}

func TestUpdateForceSkipsPrompt(t *testing.T) {
	confirmCalled := false
	gw := &fakeGateway{
		getPageFn: func(id string) (confluence.Page, error) {
			return confluence.Page{ID: "p1", Title: "T", Version: confluence.Version{Number: 5, Message: ""}}, nil
		},
	}
	svc := NewPageService(gw)

	_, err := svc.Update(context.Background(), UpdateInput{
		Content: content(),
		Target:  confluence.PageRef{PageID: "p1", SpaceKey: "DEV"},
		Opts: WriteOptions{SkipConfirm: true, Confirm: func(string) (bool, error) {
			confirmCalled = true
			return false, nil
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmCalled {
		t.Error("--force/--yes should skip the confirmation prompt")
	}
	if len(gw.updated) != 1 {
		t.Fatalf("expected one update, got %+v", gw.updated)
	}
}

func TestUpdateInsertsWhenPageGone(t *testing.T) {
	gw := &fakeGateway{
		getPageFn: func(id string) (confluence.Page, error) {
			return confluence.Page{}, confluence.ErrPageNotFound
		},
	}
	svc := NewPageService(gw)

	res, err := svc.Update(context.Background(), UpdateInput{
		Content: content(),
		Target:  confluence.PageRef{PageID: "gone", SpaceKey: "DEV"},
		Opts:    WriteOptions{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreate {
		t.Fatalf("action = %s, want create (insert fallback)", res.Action)
	}
	if len(gw.created) != 1 || gw.created[0].SpaceID != "100" {
		t.Fatalf("expected insert into resolved space, got %+v", gw.created)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a warning about the insert fallback")
	}
}

func TestDeleteConfirmAndDecline(t *testing.T) {
	newGateway := func() *fakeGateway {
		return &fakeGateway{getPageFn: func(id string) (confluence.Page, error) {
			return confluence.Page{ID: "p1", Title: "T"}, nil
		}}
	}

	t.Run("confirmed", func(t *testing.T) {
		gw := newGateway()
		res, err := NewPageService(gw).Delete(context.Background(), DeleteInput{
			Target: confluence.PageRef{PageID: "p1"},
			Opts:   WriteOptions{Confirm: alwaysYes},
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Action != ActionDelete || len(gw.deleted) != 1 {
			t.Fatalf("expected delete, got action=%s deleted=%v", res.Action, gw.deleted)
		}
	})

	t.Run("declined", func(t *testing.T) {
		gw := newGateway()
		res, err := NewPageService(gw).Delete(context.Background(), DeleteInput{
			Target: confluence.PageRef{PageID: "p1"},
			Opts:   WriteOptions{Confirm: alwaysNo},
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Action != ActionAborted || len(gw.deleted) != 0 {
			t.Fatalf("expected abort, got action=%s deleted=%v", res.Action, gw.deleted)
		}
	})
}

func TestDryRunDoesNotWrite(t *testing.T) {
	gw := &fakeGateway{}
	res, err := NewPageService(gw).Create(context.Background(), CreateInput{
		Content: content(),
		Parent:  confluence.PageRef{SpaceKey: "DEV"},
		Opts:    WriteOptions{DryRun: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.Action != ActionCreate {
		t.Fatalf("expected dry-run create, got %+v", res)
	}
	if len(gw.created) != 0 {
		t.Fatalf("dry-run must not write, got %+v", gw.created)
	}
}

func TestPageServiceView(t *testing.T) {
	t.Run("returns the page with its stored body", func(t *testing.T) {
		gw := &fakeGateway{getPageFn: func(id string) (confluence.Page, error) {
			return confluence.Page{
				ID:      id,
				Title:   "Onboarding",
				SpaceID: "100",
				Version: confluence.Version{Number: 3, Message: versionStamp},
				Body:    "<p>hello</p>",
			}, nil
		}}
		page, err := NewPageService(gw).View(context.Background(), confluence.PageRef{PageID: "120033"})
		if err != nil {
			t.Fatal(err)
		}
		if page.ID != "120033" || page.Title != "Onboarding" {
			t.Errorf("page = %+v", page)
		}
		if page.Body != "<p>hello</p>" {
			t.Errorf("body = %q, want the stored body carried through", page.Body)
		}
	})

	t.Run("a reference without a page id is refused", func(t *testing.T) {
		// A space URL names no single page, so there is nothing to show.
		_, err := NewPageService(&fakeGateway{}).View(context.Background(), confluence.PageRef{SpaceKey: "DEV"})
		if err == nil || !strings.Contains(err.Error(), "page id") {
			t.Errorf("error = %v, want it to ask for a page id", err)
		}
	})

	t.Run("view never writes", func(t *testing.T) {
		gw := &fakeGateway{getPageFn: func(id string) (confluence.Page, error) {
			return confluence.Page{ID: id}, nil
		}}
		if _, err := NewPageService(gw).View(context.Background(), confluence.PageRef{PageID: "1"}); err != nil {
			t.Fatal(err)
		}
		if len(gw.created) != 0 || len(gw.updated) != 0 || len(gw.deleted) != 0 {
			t.Errorf("view touched the gateway's write paths: %+v", gw)
		}
	})
}
