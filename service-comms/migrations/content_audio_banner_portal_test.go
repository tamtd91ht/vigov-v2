package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migrations 0012 (category `hidden`, audio and banner columns) and 0013 (portal
// sync). Like content_item_media_test.go they read the SQL this binary embeds; they do not prove
// PostgreSQL enforces it — the pg suites SKIP without VIGOV_TEST_DSN.

const (
	file0012 = "0012_content_item_audio_banner.sql"
	file0013 = "0013_portal_sync.sql"
)

// THE MUTATIONS THAT MUST TURN THIS RED: a per-type CHECK dropped or loosened; the audio pair no longer
// all-or-none; a protocol-relative or non-https link admitted; the audio check pointing at the cover
// purpose; the banner rule turned into a CHECK that validates legacy rows; `hidden` made nullable.
func TestMigration0012AudioBannerHidden(t *testing.T) {
	sql := executableSQL(t, file0012)
	for _, c := range []struct{ want, why string }{
		{"alter table danh_muc_mini_app add column if not exists hidden boolean not null default false;", "hidden is NOT NULL, false on every existing row"},
		{"foreign key (tenant_id, audio_file_id) references stored_file (tenant_id, id)", "audio FK composite with tenant_id"},
		{"check (loai = 'truyen-thanh' or (audio_file_id is null and audio_duration_seconds is null))", "audio only on truyen-thanh"},
		{"check ((audio_file_id is null) = (audio_duration_seconds is null))", "audio file and duration all or none"},
		{"audio_duration_seconds between 1 and 21600", "duration bounded at 6 h"},
		{"check (loai = 'banner' or (link_to is null and display_order is null))", "banner fields only on banner"},
		{"char_length(link_to) <= 500", "bounded link"},
		{"link_to !~ '[[:space:][:cntrl:]\\\\]'", "no whitespace, control char or backslash"},
		{"link_to ~ '^/([^/].*)?$'", "in-app path, never protocol-relative"},
		{"link_to ~* '^https://[^/?#@]+([/?#].*)?$'", "https only, no userinfo"},
		{"check (display_order is null or display_order >= 0)", "non-negative order"},
		{"on noi_dung_mini_app (tenant_id, loai, display_order, id) where deleted_at is null", "banner strip index leads with tenant_id"},
		{"or file_row.subject_id <> new.id or file_row.purpose <> 'content-audio' then", "audio uploaded for this item as audio"},
		{"before insert or update of audio_file_id on noi_dung_mini_app for each row execute function noi_dung_mini_app_audio_file_check()", "audio floor"},
		{"before insert or update of loai, cover_image_file_id on noi_dung_mini_app for each row execute function noi_dung_mini_app_banner_cover_required()", "banner cover floor"},
		{"if new.loai = 'banner' and new.cover_image_file_id is null and (tg_op = 'insert' or old.loai is distinct from 'banner' or old.cover_image_file_id is not null) then", "refuses creating, converting to, or uncovering a coverless banner"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0012 lacks %q — %s", c.want, c.why)
		}
	}
	for _, col := range []string{"audio_file_id text", "audio_duration_seconds int", "link_to text", "display_order int"} {
		if !strings.Contains(sql, "alter table noi_dung_mini_app add column if not exists "+col+";") {
			t.Errorf("0012 does not add the NULLABLE column %q", col)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"update noi_dung_mini_app", "backfills"},
		{"update danh_muc_mini_app", "backfills"},
		{"not valid", "NOT VALID on a partitioned table"},
		{"check (loai <> 'banner'", "banner cover as a CHECK would validate legacy rows"},
		{"create or replace function noi_dung_mini_app_bat_bien", "the 0006/0011 immutability function is not replaced here"},
		{"alter table stored_file", "stored_file needs no change for audio"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0012 contains %q — %s", c.banned, c.why)
		}
	}
}

// THE MUTATIONS THAT MUST TURN THIS RED: a plaintext key column; a key or index without tenant_id; a
// missing partition loop; a partial unique on external_id; the run log editable after finish or
// deletable; the portal category merged into danh_muc_mini_app; the portal category made mutable.
func TestMigration0013PortalSync(t *testing.T) {
	sql := executableSQL(t, file0013)
	for _, c := range []struct{ want, why string }{
		{"api_key_sealed bytea not null", "the key is sealed, as mail_settings.password_sealed"},
		{"check (octet_length(api_key_sealed) > 29)", "envelope overhead + at least one byte"},
		{"check (provider in ('cttdt-danang'))", "one provider"},
		{"api_url ~* '^https://([a-z0-9-]+\\.)+gov\\.vn(:[0-9]{1,5})?(/.*)?$'", "https on a .gov.vn host"},
		{"publish_mode text not null default 'cho-duyet'", "cautious default"},
		{"check (publish_mode in ('cho-duyet', 'dang-thang'))", "spec's two modes"},
		{"interval_hours int not null default 6", "owner's default"},
		{"check (interval_hours between 0 and 24)", "0 = manual only"},
		{"window_days int not null default 90", "owner's default"},
		{"max_items_per_run int not null default 100", "owner's default"},
		{"keep_source_credit boolean not null default true", "owner's default"},
		{"primary key (tenant_id), constraint portal_sync_settings_provider_known", "one row per commune"},
		{"before update or delete on portal_sync_settings for each row execute function portal_sync_settings_guard()", "settings never deleted"},
		{"primary key (tenant_id, id), unique (tenant_id, external_id),", "category keys composite with tenant_id, not partial"},
		{"check (target_kind in ('tin-tuc', 'su-kien', 'thong-bao'))", "three target kinds"},
		{"before update or delete on portal_categories for each row execute function portal_categories_guard()", "category guard"},
		{"or new.external_id is distinct from old.external_id", "external id frozen"},
		{"check (trigger_kind in ('theo-lich', 'chay-tay'))", "what started the run (ADR 0011 values)"},
		{"(trigger_kind = 'theo-lich' and actor = 'system') or (trigger_kind = 'chay-tay' and actor <> 'system' and btrim(actor) <> '')", "actor matches trigger"},
		{"jsonb_typeof(error_summary) = 'array' and octet_length(error_summary::text) <= 65536", "bounded error summary"},
		{"on portal_sync_runs (tenant_id, started_at desc, id)", "run history index leads with tenant_id"},
		{"before delete on portal_sync_runs for each row execute function ho_so_luu_tru_cam_xoa_cung()", "runs never deleted"},
		{"before update on portal_sync_runs for each row execute function portal_sync_run_finish_once()", "finish filled once"},
		{"foreign key (tenant_id, portal_category_id) references portal_categories (tenant_id, id)", "item → portal category, composite"},
		{"check (portal_category_id is null or nguon = 'dong-bo-cong')", "only synced items carry a portal category"},
		{"before update of portal_category_id on noi_dung_mini_app for each row execute function noi_dung_mini_app_portal_category_frozen()", "write-once provenance"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0013 lacks %q — %s", c.want, c.why)
		}
	}
	for _, table := range []string{"portal_sync_settings", "portal_categories", "portal_sync_runs"} {
		if !strings.Contains(sql, "partition of "+table+" ' 'for values with (modulus 32, remainder %s)") {
			t.Errorf("0013: %s has no MODULUS 32 partition loop", table)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"delete from", "deletes rows"},
		{"not valid", "NOT VALID on a partitioned table"},
		{"danh_muc_mini_app", "portal categories are never merged with, or joined to, the commune's own tree (0006:141-146)"},
		{"api_key text", "plaintext key"},
		{"ma_bao_mat", "plaintext key under the spec's name"},
		{"create or replace function noi_dung_mini_app_bat_bien", "the 0006/0011 immutability function is not replaced here"},
		{"where deleted_at is null", "no soft delete in these tables; a partial key would reissue"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0013 contains %q — %s", c.banned, c.why)
		}
	}
}
