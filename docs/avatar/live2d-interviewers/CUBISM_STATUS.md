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

- `frontend/public/live2d/interviewers/cubism-input/female/female-neutral-2048.png`
  is ready for import.
- No `.cmo3` project or rig exists yet.

## Export and validation

- Texture atlases have not been generated.
- No authentic `.moc3`, `.model3.json`, or `.cdi3.json` has been exported.
- `check:live2d` cannot pass until authentic Cubism runtime files exist.
- No placeholder or validation-bypass files have been created.

## Remaining work in Cubism

1. Import the eye-closed and mouth-open difference images as aligned ArtMeshes,
   or replace the current flat source with a genuinely layered PSD.
2. Add opacity/deformation keyforms for the four required parameters from
   `rigging-spec.json`.
3. Register the LipSync and EyeBlink groups.
4. Generate a single 2048 px texture atlas.
5. Export the male runtime files, then repeat the process for the female model.
6. Run `npm run check:live2d` for both exports and inspect them at 96–120 px.
