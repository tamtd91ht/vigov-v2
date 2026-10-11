/**
 * "ĐỊA CHỈ NƠI XẢY RA SỰ VIỆC" ON THE COMMUNE APP'S SEND FORM — the prototype's block
 * (`../vigov-require/apps/miniapp/src/components/feedback/AddressBlock.tsx`, owner 11/10/2026: theo prototype):
 * a title with "(không bắt buộc)", one hint, a two-line box, the location line and its button, and a small map.
 *
 * THE BOX IS WHAT THE CITIZEN TYPED, NEVER A GUESS: the coordinates are shown on their own line, not written into
 * the box (officers need "trước số nhà 112", which only the person on the spot can say — the prototype's reason).
 *
 * THE MAP IS A DRAWING, NOT A MAP: inline SVG streets and blocks with a red pin in the middle once a location is
 * known, and the prototype's caption saying it only illustrates. No tile server, no third-party script, no request
 * of any kind — so the coordinates never leave the phone except inside the petition the citizen sends (rule 3).
 *
 * ⚠ NOTHING HERE LOGS, STORES OR PUTS COORDINATES IN A URL OR A KEY (as `scene-location.tsx`).
 */
import type { SceneLocation } from "../api/hop-dong-phan-anh";
import type { ZaloFailure } from "../api/mo-phien-vigov";

import { BieuTuong } from "./BieuTuong";
import { XA_PA, zaloSupportCode } from "./noi-dung";
import { ONhapDoan } from "./o-nhap"; // vi-name-ok: existing export, not renamed (rule 12 #3)
import {
  formatCoordinates,
  type SceneLocationFailure,
  sceneLocationFailureText,
  type SceneLocationWords,
} from "./scene-location";

/**
 * The prototype's sketch (`AddressBlock.tsx:136-171`): streets and blocks in the page's blue-greys, the pin only
 * when a location is known and not being re-taken. Decoration (`aria-hidden`) — the coordinates line says it in
 * words; the caption is real text.
 */
export function MiniMap({ active }: { active: boolean }) {
  return (
    <div className="xa-mini-map">
      <div className="xa-mini-map__canvas">
        <svg viewBox="0 0 300 112" className="xa-mini-map__svg" aria-hidden="true" focusable="false" preserveAspectRatio="xMidYMid slice">
          <rect width="300" height="112" fill="#E8F0F4" />
          <path d="M0 74 H300" stroke="#CBD9E2" strokeWidth="12" />
          <path d="M0 30 H300" stroke="#D8E4EC" strokeWidth="7" />
          <path d="M96 0 V112" stroke="#CBD9E2" strokeWidth="10" />
          <path d="M212 0 V112" stroke="#D8E4EC" strokeWidth="6" />
          <rect x="18" y="40" width="60" height="26" rx="3" fill="#DCE7EE" />
          <rect x="112" y="38" width="80" height="30" rx="3" fill="#DCE7EE" />
          <rect x="228" y="42" width="54" height="24" rx="3" fill="#DCE7EE" />
          <rect x="120" y="82" width="70" height="22" rx="3" fill="#DCE7EE" />
        </svg>
        {active && (
          <span className="xa-mini-map__pin" aria-hidden="true">
            <svg width="34" height="34" viewBox="0 0 24 24" focusable="false">
              <path
                d="M12,21s6.8,-6.3,6.8,-10.8A6.8,6.8,0,0,0,5.2,10.2C5.2,14.7,12,21,12,21z"
                fill="#e5484d"
                stroke="#c0272c"
                strokeWidth="2.5"
                strokeLinejoin="round"
              />
              <circle cx="12" cy="10.2" r="2.4" fill="#ffffff" />
            </svg>
          </span>
        )}
      </div>
      <p className="xa-mini-map__caption">{XA_PA.map_caption}</p>
    </div>
  );
}

/**
 * The location line under the box: "Đang lấy vị trí…" while it runs; the coordinates once known; the failure in
 * words (and Zalo's "Mã hỗ trợ" under it); then the button — a full 48px target in link style, with words.
 * `role="status"`: each change is read out.
 */
export function LocationStatus(props: {
  words: SceneLocationWords;
  locating: boolean;
  location: SceneLocation | null;
  failure: SceneLocationFailure | null;
  zalo: ZaloFailure | null;
  onLocate: () => void;
}) {
  const { words } = props;
  if (props.locating) {
    return (
      <p className="xa-location-status" role="status">
        {words.locating}
      </p>
    );
  }
  return (
    <>
      {props.location !== null && (
        <p className="xa-location-status" role="status">
          <BieuTuong ten="pin" co={18} />
          <span>{words.found(formatCoordinates(props.location))}</span>
        </p>
      )}
      {props.failure !== null && (
        <p className="xa-loi-o" role="status">
          {sceneLocationFailureText(words, props.failure, props.zalo)}
        </p>
      )}
      {props.failure !== null && zaloSupportCode(props.zalo) !== null && <p className="xa-phu">{zaloSupportCode(props.zalo)}</p>}
      <button type="button" className="xa-location-button" onClick={props.onLocate}>
        <BieuTuong ten="locate-fixed" co={20} />
        {props.location !== null ? words.button_again : words.button}
      </button>
    </>
  );
}

export function AddressBlock(props: {
  address: string;
  maxLength: number;
  /** The box's own check (too long), or none. */
  error?: string;
  onAddress: (value: string) => void;
  /** The shell gave a location bridge (inside Zalo). Without one there is no button and no location line. */
  canLocate: boolean;
  words: SceneLocationWords;
  locating: boolean;
  location: SceneLocation | null;
  failure: SceneLocationFailure | null;
  zalo: ZaloFailure | null;
  onLocate: () => void;
}) {
  return (
    <section className="xa-address" aria-labelledby="xa-address-title">
      <h2 className="xa-form-title" id="xa-address-title">
        {XA_PA.address_title} <span className="xa-form-title__optional">{XA_PA.optional_suffix}</span>
      </h2>
      <p className="xa-phu">{XA_PA.address_hint}</p>
      <div className="xa-the xa-address__card">
        <div className="xa-address__body">
          <ONhapDoan
            id="xa-dia-chi"
            nhan={XA_PA.address_title}
            labelHidden
            placeholder={XA_PA.address_placeholder}
            rows={2}
            gia_tri={props.address}
            toi_da={props.maxLength}
            onDoi={props.onAddress}
          />
          {props.error !== undefined && (
            <p className="xa-loi-o" role="alert">
              {props.error}
            </p>
          )}
          {props.canLocate && (
            <LocationStatus
              words={props.words}
              locating={props.locating}
              location={props.location}
              failure={props.failure}
              zalo={props.zalo}
              onLocate={props.onLocate}
            />
          )}
        </div>
        <MiniMap active={!props.locating && props.location !== null} />
      </div>
    </section>
  );
}
