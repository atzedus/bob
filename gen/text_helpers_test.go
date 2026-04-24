package gen

import (
	"testing"

	"github.com/stephenafamo/bob/orm"
)

func TestSelfJoinBackReferenceConfig_Apply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  SelfJoinBackReferenceConfig
		fwd  string
		want string
	}{
		{
			name: "zero value applies historical default",
			cfg:  SelfJoinBackReferenceConfig{},
			fwd:  "ParentRecords",
			want: "ReverseParentRecords",
		},
		{
			name: "explicit defaults match zero value",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Reverse",
				Joiner:   "",
				Position: SelfJoinPositionPrefix,
			},
			fwd:  "ParentRecords",
			want: "ReverseParentRecords",
		},
		{
			name: "children + by + prefix",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Children",
				Joiner:   "By",
				Position: SelfJoinPositionPrefix,
			},
			fwd:  "ParentRecords",
			want: "ChildrenByParentRecords",
		},
		{
			name: "children + suffix",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Children",
				Position: SelfJoinPositionSuffix,
			},
			fwd:  "ParentRecord",
			want: "ParentRecordChildren",
		},
		{
			name: "dependents + suffix",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Dependents",
				Position: SelfJoinPositionSuffix,
			},
			fwd:  "ParentRecords",
			want: "ParentRecordsDependents",
		},
		{
			name: "joiner is preserved verbatim in suffix position",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Children",
				Joiner:   "Of",
				Position: SelfJoinPositionSuffix,
			},
			fwd:  "ParentRecords",
			want: "ParentRecordsOfChildren",
		},
		{
			name: "unknown position falls back to prefix",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Reverse",
				Position: SelfJoinPosition("bogus"),
			},
			fwd:  "ParentRecords",
			want: "ReverseParentRecords",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.Apply(tt.fwd)
			if got != tt.want {
				t.Fatalf("Apply(%q): want %q, got %q", tt.fwd, tt.want, got)
			}
		})
	}
}

func TestSelfJoinBackReferenceConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     SelfJoinBackReferenceConfig
		wantErr bool
	}{
		{
			name:    "default is valid",
			cfg:     SelfJoinBackReferenceConfig{}.WithDefaults(),
			wantErr: false,
		},
		{
			name: "valid prefix configuration",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Children",
				Joiner:   "By",
				Position: SelfJoinPositionPrefix,
			},
			wantErr: false,
		},
		{
			name: "valid suffix configuration",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Dependents",
				Position: SelfJoinPositionSuffix,
			},
			wantErr: false,
		},
		{
			name: "invalid position",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Reverse",
				Position: SelfJoinPosition("middle"),
			},
			wantErr: true,
		},
		{
			name: "empty token after defaults is rejected",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "",
				Position: SelfJoinPositionPrefix,
			},
			// WithDefaults backfills Token, but Validate is documented to be
			// callable independently — invoking it on a raw value with an
			// empty token must surface an error so misuse is caught.
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate(): wantErr=%v, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestRelAlias_SelfJoinBackReference exercises relAlias end-to-end to make
// sure the config actually drives the alias of the synthetic reverse
// self-join relationship, while leaving forward sides and ordinary FKs
// untouched.
func TestRelAlias_SelfJoinBackReference(t *testing.T) {
	t.Parallel()

	// Self-referencing FK on tree.parent_record_id -> tree.id.
	// Side flags mirror what initRelationships+inferModify produce for the
	// generator: forward is many-to-one (Modify=from, ToUnique=true),
	// reverse is one-to-many (Modify=to, ToUnique=false).
	forward := orm.Relationship{
		Name: "tree_parent_id_fkey",
		Sides: []orm.RelSide{{
			From:        "tree",
			To:          "tree",
			FromColumns: []string{"parent_record_id"},
			ToColumns:   []string{"id"},
			Modify:      "from",
			ToUnique:    true,
		}},
	}
	reverse := orm.Relationship{
		Name: "tree_parent_id_fkey" + selfJoinSuffix,
		Sides: []orm.RelSide{{
			From:        "tree",
			To:          "tree",
			FromColumns: []string{"id"},
			ToColumns:   []string{"parent_record_id"},
			Modify:      "to",
			FromUnique:  true,
		}},
	}

	// Ordinary FK on order_items.order_id -> orders.id.
	// Used to verify the config does NOT affect non self-referencing FKs.
	ordinary := orm.Relationship{
		Name: "order_items_order_id_fkey",
		Sides: []orm.RelSide{{
			From:        "order_items",
			To:          "orders",
			FromColumns: []string{"order_id"},
			ToColumns:   []string{"id"},
			Modify:      "from",
			ToUnique:    true,
		}},
	}

	tests := []struct {
		name        string
		cfg         SelfJoinBackReferenceConfig
		wantReverse string
	}{
		{
			name:        "default preserves ReverseParentRecords",
			cfg:         SelfJoinBackReferenceConfig{},
			wantReverse: "ReverseParentRecords",
		},
		{
			name: "children + by + prefix",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Children",
				Joiner:   "By",
				Position: SelfJoinPositionPrefix,
			},
			wantReverse: "ChildrenByParentRecords",
		},
		{
			name: "dependents + suffix",
			cfg: SelfJoinBackReferenceConfig{
				Token:    "Dependents",
				Position: SelfJoinPositionSuffix,
			},
			wantReverse: "ParentRecordsDependents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selfRels := []orm.Relationship{forward, reverse}
			got := relAlias(selfRels, tt.cfg)

			if got[forward.Name] != "ParentRecord" {
				t.Errorf(
					"forward side alias must not be affected by self_join_back_reference: want %q, got %q",
					"ParentRecord", got[forward.Name],
				)
			}
			if got[reverse.Name] != tt.wantReverse {
				t.Errorf(
					"reverse self-join alias: want %q, got %q",
					tt.wantReverse, got[reverse.Name],
				)
			}

			// Ordinary FK alias derives from the foreign table name and
			// must be completely independent of the self-join setting.
			ord := relAlias([]orm.Relationship{ordinary}, tt.cfg)
			if ord[ordinary.Name] != "Order" {
				t.Errorf(
					"ordinary FK alias must be unaffected by self_join_back_reference: want %q, got %q",
					"Order", ord[ordinary.Name],
				)
			}
		})
	}
}
