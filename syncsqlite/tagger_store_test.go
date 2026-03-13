package syncsqlite

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jfardello/tdns/storage"
)

func TestSQLiteStorage_LabelAndMemberLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tagger.sqlite")
	store, err := NewSQLiteStorage(storage.WithDbPath(dbPath))
	if err != nil {
		t.Fatalf("NewSQLiteStorage error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close error: %v", err)
		}
	}()

	if err := store.CreateLabel("red"); err != nil {
		t.Fatalf("CreateLabel error: %v", err)
	}
	if err := store.AddMembersToLabel("red", []string{"1.1.1.1", "2.2.2.2", "1.1.1.1"}); err != nil {
		t.Fatalf("AddMembersToLabel error: %v", err)
	}
	if err := store.ReplaceMemberLabels("3.3.3.3", []string{"blue", "green", "blue"}); err != nil {
		t.Fatalf("ReplaceMemberLabels error: %v", err)
	}
	if err := store.RemoveMemberFromLabel("red", "2.2.2.2"); err != nil {
		t.Fatalf("RemoveMemberFromLabel error: %v", err)
	}

	labels, err := store.GetLabels()
	if err != nil {
		t.Fatalf("GetLabels error: %v", err)
	}
	wantLabels := []string{"blue", "green", "red"}
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Fatalf("GetLabels got %v, want %v", labels, wantLabels)
	}

	redMembers, err := store.GetLabelMembers("red")
	if err != nil {
		t.Fatalf("GetLabelMembers error: %v", err)
	}
	wantRedMembers := []string{"1.1.1.1"}
	if !reflect.DeepEqual(redMembers, wantRedMembers) {
		t.Fatalf("GetLabelMembers got %v, want %v", redMembers, wantRedMembers)
	}

	memberLabels, err := store.GetMemberLabels("3.3.3.3")
	if err != nil {
		t.Fatalf("GetMemberLabels error: %v", err)
	}
	wantMemberLabels := []string{"blue", "green"}
	if !reflect.DeepEqual(memberLabels, wantMemberLabels) {
		t.Fatalf("GetMemberLabels got %v, want %v", memberLabels, wantMemberLabels)
	}

	if err := store.DeleteLabel("green"); err != nil {
		t.Fatalf("DeleteLabel error: %v", err)
	}
	memberLabels, err = store.GetMemberLabels("3.3.3.3")
	if err != nil {
		t.Fatalf("GetMemberLabels after DeleteLabel error: %v", err)
	}
	if !reflect.DeepEqual(memberLabels, []string{"blue"}) {
		t.Fatalf("GetMemberLabels after DeleteLabel got %v, want %v", memberLabels, []string{"blue"})
	}

	if err := store.DeleteMember("3.3.3.3"); err != nil {
		t.Fatalf("DeleteMember error: %v", err)
	}
	memberLabels, err = store.GetMemberLabels("3.3.3.3")
	if err != nil {
		t.Fatalf("GetMemberLabels after DeleteMember error: %v", err)
	}
	if len(memberLabels) != 0 {
		t.Fatalf("expected deleted member to have no labels, got %v", memberLabels)
	}
}
