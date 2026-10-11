/**
 * "ĐỊA CHỈ NƠI XẢY RA SỰ VIỆC" ON THE COMMUNE APP'S SEND FORM — the prototype's block
 * (`../vigov-require/apps/miniapp/src/components/feedback/AddressBlock.tsx`, owner 11/10/2026: theo prototype):
 * a title with "(không bắt buộc)", one hint, a two-line box, the location line and its button, and a small map.
 *
 * THE BOX IS WHAT THE CITIZEN TYPED, NEVER A GUESS: the coordinates are shown on their own line, not written into
 * the box (officers need "trước số nhà 112", which only the person on the spot can say — the prototype's reason).
 *
 * THE MAP IS REAL since 11/10/2026 (owner; ADR 0072 §Sửa đổi 11/10/2026 (Mini App)) — `../map/scene-map.tsx`, on the
 * self-hosted basemap through ViGov's `comms`, locked to the commune's frame. The SVG sketch it replaces is gone. With
 * no map (no frame, no route, failure, accountless path, outside Zalo) the block is today's: box, line, button, plus
 * at most one sentence — and the send never waits for the map.
 *
 * ⚠ NOTHING HERE LOGS, STORES OR PUTS COORDINATES IN A URL OR A KEY (as `scene-location.tsx`). The tile requests of
 *   the map name the viewport; that is declared (M5, `content/ket-xuat-ho-so.ts`).
 */
import type { SceneLocation } from "../api/hop-dong-phan-anh";
import type { ZaloFailure } from "../api/mo-phien-vigov";
import { SceneMapArea } from "../map/scene-map";
import type { SceneMapState } from "../map/scene-map-logic";

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
  /** The citizen placed the point on the map: the line says so, so Zalo's point and theirs never read alike. */
  placedOnMap?: boolean;
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
          <span>
            {props.placedOnMap === true
              ? XA_PA.map_placed(formatCoordinates(props.location))
              : words.found(formatCoordinates(props.location))}
          </span>
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
  /**
   * The real map (`../map/scene-map.tsx`), or absent = no map area at all (the accountless path cannot send a point;
   * outside Zalo there is none to show). `placedOnMap`: the current location came from the pin, not from Zalo.
   */
  map?: {
    state: SceneMapState;
    placedOnMap: boolean;
    onPlace: (location: SceneLocation) => void;
    onFail: () => void;
  };
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
              placedOnMap={props.map?.placedOnMap === true}
            />
          )}
        </div>
        {props.map !== undefined && (
          <SceneMapArea
            state={props.map.state}
            location={props.locating ? null : props.location}
            onPlace={props.map.onPlace}
            onFail={props.map.onFail}
          />
        )}
      </div>
    </section>
  );
}
