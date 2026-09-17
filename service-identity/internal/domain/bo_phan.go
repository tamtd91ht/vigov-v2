package domain

// BoPhan is one node of the commune's organisational chart.
//
// THE URL RESOURCE IS `org-units`, NOT `departments`, and the reason is business rather than
// stylistic: this tree holds Đảng uỷ, HĐND and UBMTTQ alongside the UBND's own units, so
// "department" would assert that every node is a department of the People's Committee — which is
// untrue of most of them. The mapping is owned by kb/00-foundation/ubiquitous-language.md (ADR
// 0011); this comment points at it rather than restating the table (rule 9).
type BoPhan struct {
	ID  string // ULID, internal
	Ma  string // slug: "van-phong-dang-uy"
	Ten string // "VĂN PHÒNG ĐẢNG ỦY" — upper-case by the commune's own convention, kept as typed

	// ChaID is the parent node, "" at the root.
	//
	// IT IS RETURNED RATHER THAN RESOLVED INTO A NESTED TREE. A flat list with parent ids is what
	// every consumer of this can use: the filter box wants the names, the assignment box wants the
	// leaves, and only the org-chart screen wants the shape. Nesting it here would force the two
	// consumers that do not want a tree to walk one, and would make the response shape depend on
	// the data's depth — a list of 10 rows is a list of 10 rows whatever the tree looks like.
	//
	// "" AND NOT NULL: the column is nullable and the query coalesces, which keeps one type at
	// every layer. A root node is the one with no parent, which is the same statement either way.
	ChaID string
}
