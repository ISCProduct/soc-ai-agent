# Live2D interviewer rigging status

Last checked: 2026-11-14

## Male model

- Source project confirmed at `~/Desktop/Untitled Model.cmo3`.
- A copy is kept in S3 at `live2d-authoring/.../cubism-wip/male-interviewer.cmo3`.
- The Cubism project opens successfully in Cubism Editor 5.3.04 FREE.
- The standard parameters include `ParamAngleY`, `ParamEyeLOpen`,
  `ParamEyeROpen`, `ParamMouthOpenY`, `ParamAngleX`, and `ParamBreath`.
- Named part folders exist for the face, eyes, pupils, eyebrows, mouth, nose,
  ears, hair, neck, and body.
- The editable project is saved at `cubism-wip/male-interviewer.cmo3`
  (fetch it with `make live2d-authoring`; it is no longer in Git).
- An authentic Cubism runtime export exists at
  `frontend/public/live2d/interviewers/male/`.

### Blocking source issue

The named part folders are empty. The model currently uses three ArtMeshes
(neutral / eyes-closed / mouth-open) packed into one 2048 px atlas, but the
opacity keyforms for `ParamMouthOpenY` / `ParamEyeLOpen` /
`ParamEyeROpen` are not in the exported `.moc3`. The app therefore drives
those ArtMeshes from code (`live2d-overlay.ts`) as a temporary measure.

## Female model

- `female-neutral-2048.png` was imported into Cubism Editor.
- The editable project is saved at `cubism-wip/female-interviewer.cmo3`
  (fetch it with `make live2d-authoring`; it is no longer in Git).
  The S3 project is the **3 ArtMesh** version (neutral / eyes-closed /
  mouth-open), matching `docs/wiki/live2d-assets.md`.
- An authentic Cubism runtime export exists at
  `frontend/public/live2d/interviewers/female/`.
- Opacity keys for `ParamMouthOpenY` and `ParamEyeLOpen` are set in Cubism;
  `ParamEyeROpen` is in the EyeBlink group but shares the left-eye difference
  ArtMesh (no independent right-eye key).

## Export and validation

- Authentic `.moc3`, `.model3.json`, `.cdi3.json`, and texture-atlas files
  were exported for both models by Cubism Editor 5.3.04 FREE.
- Female re-export uses a single 2048 px atlas; male still needs keyforms
  reflected into the `.moc3` (see Remaining work).
- `npm run check:live2d -- public/live2d/interviewers/male` and
  `.../female` both report usable as interviewer (parameter / LipSync /
  EyeBlink checks only — not motion proof).
- No placeholder or validation-bypass files were created.

## Remaining work in Cubism

1. Male: set opacity keyforms for mouth-open and eyes-closed ArtMeshes on
   `ParamMouthOpenY` / `ParamEyeLOpen` / `ParamEyeROpen`, then re-export so
   the code-side overlay can be removed.
2. Female: final visual check at 96–120 px (240 px already confirmed).
   Optionally add an independent `ParamEyeROpen` key.
3. Optional quality upgrade: replace flat sources with a layered PSD and
   rig parts properly.
4. After any re-export, confirm ArtMesh IDs and update
   `frontend/lib/interview/live2d-overlay.ts` if they changed.
