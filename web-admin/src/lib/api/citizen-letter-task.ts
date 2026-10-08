/**
 * POST /api/v1/citizen-letter-tasks — "Chuyển thành nhiệm vụ" on the citizen-letter drawer (ADR 0085 A,
 * §Trả lời; ADR 0084 #6). Owner: `service-petitions` (`internal/http/citizen_letter_task.go`), which asks
 * `service-documents` about the letter. `task.create` AND `petition.read`; 201 with the new task.
 *
 * WHAT THIS FILE NEVER SENDS — and the server answers 400 to each, by name:
 *   - `source`, `source_id`: the source IS the letter (`letter_id`), set by the server;
 *   - `due_at`: the task inherits the letter's current deadline (its date, 17:00), computed by the
 *     server from the stored value — the dialog only SHOWS it (rule 10, invariant 2).
 * The body is therefore built field by field, never spread: a spread is how a `due_at` from the
 * create-task form would reach the server.
 *
 * Refusals come back VERBATIM (`thongBao`): 404 one sentence for unknown / removed / another commune's
 * letter; 422 `denunciation_no_task` (C9) and `assignment_required`; 503 when documents could not be asked
 * — nothing written in any of them.
 *
 * `idempotencyKey` IS A PARAMETER, minted when the dialog OPENS (rule 7, invariant 3): a retry after a
 * lost answer must replay — the server then answers the SAME task's code — never book a second task.
 */

import {
  docThanLoiGoi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  goiGhi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  type KetQua, // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
} from "./goi";
import type {
  petitions_citizenLetterTaskIn,
  petitions_nhiemVuRa,
  petitions_post_citizen_letter_tasks,
  petitions_taoNhiemVuVao,
  petitions_vanBanNhiemVuVao,
} from "./schema.gen";

/**
 * What the builder reads: the keys the create-task form and this route SHARE. `petitions_taoNhiemVuVao`
 * (the form's body, with `due_at` / `source`) and `petitions_citizenLetterTaskIn` both fit it.
 */
export type LetterTaskFields = Omit<petitions_taoNhiemVuVao, "due_at" | "source" | "source_id">;

/**
 * The dialog's answer → the route's body. `form` is the create-task form's body (`FormGiaoViec`); only
 * the keys this route takes are copied, and the empty ones are left out (absent = "not declared").
 */
export function letterTaskBody(letterId: string, form: LetterTaskFields): petitions_citizenLetterTaskIn {
  const body: petitions_citizenLetterTaskIn = {
    letter_id: letterId,
    auto_code: form.auto_code,
    type: form.type,
    title: form.title,
  };
  if (form.code !== undefined && form.code !== "") body.code = form.code;
  if (form.bloc !== undefined && form.bloc !== "") body.bloc = form.bloc;
  if (form.description !== undefined && form.description !== "") body.description = form.description;
  if (form.priority !== undefined && form.priority !== "") body.priority = form.priority;
  if (form.note !== undefined && form.note !== "") body.note = form.note;
  if (form.unit !== undefined && form.unit !== "") body.unit = form.unit;
  if (form.assignee !== undefined && form.assignee !== "") body.assignee = form.assignee;
  if (form.assigner !== undefined && form.assigner !== "") body.assigner = form.assigner;
  if (form.parent !== undefined && form.parent !== "") body.parent = form.parent;
  if (form.documents !== undefined && form.documents.length > 0) {
    body.documents = form.documents.map((d) => {
      // No `id`: on creation every row is new (the create form's rule, `taoNhiemVu`).
      const row: petitions_vanBanNhiemVuVao = { group: d.group, summary: d.summary };
      if (d.reference !== undefined && d.reference !== "") row.reference = d.reference;
      if (d.date !== undefined && d.date !== "") row.date = d.date;
      return row;
    });
  }
  return body;
}

/** POST the body. 201 → the task (a replay carries at least its `code`). */
export function createTaskFromLetter(
  body: petitions_citizenLetterTaskIn,
  idempotencyKey: string,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const path: petitions_post_citizen_letter_tasks["duongDan"] = "/api/v1/citizen-letter-tasks";
  // Rebuilt here too: a caller holding a wider object cannot slip a key past the builder.
  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(path, "POST", letterTaskBody(body.letter_id, body), 201, { "Idempotency-Key": idempotencyKey }),
  );
}
