package domain

// DocumentTypeCodeState is one live document type of the commune, reduced to what another service
// may learn about it through ResolveDocumentTypeCodes (proto/vigov/documents/v1/documents.proto):
// its code (`loai_van_ban.ma`) and whether the commune has it switched on (`dang_dung`).
//
// Nothing else — no label, no id, no order. The caller (identity, before it writes an SLA row) needs
// one bit about a code it already holds; anything more is a copy of the catalogue nobody reviews.
type DocumentTypeCodeState struct {
	Code   string
	Active bool
}
