# Live2D interviewer rigging status

Last checked: 2026-10-28

## Male model

- Source project confirmed at `~/Desktop/Untitled Model.cmo3`.
- A repository copy is available at `cubism-wip/male-interviewer.cmo3`.
- The Cubism project opens successfully in Cubism Editor 5.3.04 FREE.
- The standard parameters include `ParamAngleY`, `ParamEyeLOpen`,
  `ParamEyeROpen`, `ParamMouthOpenY`, `ParamAngleX`, and `ParamBreath`.
- Named part folders exist for the face, eyes, pupils, eyebrows, mouth, nose,
  ears, hair, neck, and body.
- The editable project is saved at `cubism-wip/male-interviewer.cmo3`.
- An authentic Cubism runtime export exists at
  `frontend/public/live2d/interviewers/male/`.

### Blocking source issue

The named part folders are empty. The model currently contains one visible
ArtMesh, `male-neutral-2048`, for the complete character. The face, eyes, and
mouth are not independent ArtMeshes. Therefore the existing project cannot
produce independent blinking and mouth movement merely by adding deformers to
the named folders.

The supplied closed-eye and open-mouth PNGs can be imported as overlay
ArtMeshes, but their placement, opacity keyforms, atlas packing, and visual
seams must be adjusted in Cubism before the model is production-ready.

## Female model

- `female-neutral-2048.png` was imported into Cubism Editor.
- The editable project is saved at `cubism-wip/female-interviewer.cmo3`.
- An authentic Cubism runtime export exists at
  `frontend/public/live2d/interviewers/female/`.
- Like the male source, the visible character is a single flat ArtMesh; the
  eyes and mouth have not been separated and keyed.

## Export and validation

- Authentic `.moc3`, `.model3.json`, `.cdi3.json`, and 1024 px texture-atlas
  files were exported for both models by Cubism Editor 5.3.04 FREE.
- `npm run check:live2d -- public/live2d/interviewers/male` passes with a
  LipSync-group warning.
- `npm run check:live2d -- public/live2d/interviewers/female` passes with the
  same warning.
- No placeholder or validation-bypass files were created.
- Passing the structural checker does not mean the flat ArtMesh is rigged:
  moving the parameters still does not create visible blinking, mouth movement,
  or nodding.

## Remaining work in Cubism

1. Import the eye-closed and mouth-open difference images as aligned ArtMeshes,
   or replace the current flat source with a genuinely layered PSD.
2. Add opacity/deformation keyforms for the four required parameters from
   `rigging-spec.json`.
3. Register `ParamMouthOpenY` in the LipSync group and confirm both eye-open
   parameters remain in EyeBlink.
4. Regenerate the requested single 2048 px atlas (the current FREE-edition
   exports contain a 1024 px atlas).
5. Re-export both runtime models and inspect actual animation at 96–120 px.
