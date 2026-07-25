#!/usr/bin/env python3
"""
Generate humanoid avatar GLBs with Oculus Viseme morph targets.
Outputs male-avatar.glb and female-avatar.glb in frontend/public/avatars/
"""

import struct, json, math, os

# ── Oculus Viseme names ──────────────────────────────────────────────────────
OCULUS_VISEMES = [
    "viseme_sil", "viseme_PP", "viseme_FF", "viseme_TH", "viseme_DD",
    "viseme_kk", "viseme_CH", "viseme_SS", "viseme_nn", "viseme_RR",
    "viseme_aa", "viseme_E",  "viseme_I",  "viseme_O",  "viseme_U",
]

# ── Geometry helpers ─────────────────────────────────────────────────────────

def uv_sphere(cx, cy, cz, rx, ry, rz, lat=20, lon=32):
    """Scaled UV sphere. Returns verts list and flat index list."""
    verts = []
    verts.append((cx, cy + ry, cz))
    for i in range(1, lat):
        theta = math.pi * i / lat
        y = cy + ry * math.cos(theta)
        s = math.sin(theta)
        for j in range(lon):
            phi = 2 * math.pi * j / lon
            x = cx + rx * s * math.cos(phi)
            z = cz + rz * s * math.sin(phi)
            verts.append((x, y, z))
    verts.append((cx, cy - ry, cz))

    idxs = []
    for j in range(lon):
        idxs += [0, 1 + j, 1 + (j + 1) % lon]
    for i in range(lat - 2):
        for j in range(lon):
            a = 1 + i * lon + j;          b = 1 + i * lon + (j + 1) % lon
            c = 1 + (i + 1) * lon + j;    d = 1 + (i + 1) * lon + (j + 1) % lon
            idxs += [a, b, d, a, d, c]
    bot = len(verts) - 1
    for j in range(lon):
        a = 1 + (lat - 2) * lon + j
        b = 1 + (lat - 2) * lon + (j + 1) % lon
        idxs += [bot, b, a]
    return verts, idxs


def cylinder(cx, cy_bot, cz, r_bot, r_top, h, segs=20, close_top=True, close_bot=True):
    verts = []
    idxs  = []
    for j in range(segs):
        phi = 2 * math.pi * j / segs
        verts.append((cx + r_bot * math.cos(phi), cy_bot,     cz + r_bot * math.sin(phi)))
    for j in range(segs):
        phi = 2 * math.pi * j / segs
        verts.append((cx + r_top * math.cos(phi), cy_bot + h, cz + r_top * math.sin(phi)))
    for j in range(segs):
        a = j; b = (j + 1) % segs; c = segs + j; d = segs + (j + 1) % segs
        idxs += [a, c, d, a, d, b]
    if close_bot:
        cb = len(verts); verts.append((cx, cy_bot, cz))
        for j in range(segs):
            idxs += [cb, (j + 1) % segs, j]
    if close_top:
        ct = len(verts); verts.append((cx, cy_bot + h, cz))
        for j in range(segs):
            idxs += [ct, segs + j, segs + (j + 1) % segs]
    return verts, idxs


def merge(parts):
    """Merge list of (verts, idxs) into single arrays."""
    all_v, all_i = [], []
    offset = 0
    for v, i in parts:
        all_v.extend(v)
        all_i.extend(x + offset for x in i)
        offset += len(v)
    return all_v, all_i


def compute_normals(verts, idxs):
    n = [None] * len(verts)
    acc = [[0.0, 0.0, 0.0] for _ in verts]
    for k in range(0, len(idxs), 3):
        a, b, c = idxs[k], idxs[k + 1], idxs[k + 2]
        va, vb, vc = verts[a], verts[b], verts[c]
        ab = (vb[0]-va[0], vb[1]-va[1], vb[2]-va[2])
        ac = (vc[0]-va[0], vc[1]-va[1], vc[2]-va[2])
        nx = ab[1]*ac[2] - ab[2]*ac[1]
        ny = ab[2]*ac[0] - ab[0]*ac[2]
        nz = ab[0]*ac[1] - ab[1]*ac[0]
        for vi in (a, b, c):
            acc[vi][0] += nx; acc[vi][1] += ny; acc[vi][2] += nz
    for i, (nx, ny, nz) in enumerate(acc):
        L = math.sqrt(nx*nx + ny*ny + nz*nz) or 1.0
        n[i] = (nx/L, ny/L, nz/L)
    return n


# ── Mouth morph target helper ────────────────────────────────────────────────

def mouth_mask(verts, head_cx, head_cy, head_cz, head_rx, head_ry, head_rz):
    """Return list of (idx, weight) for mouth-area vertices (0.0–1.0)."""
    result = []
    for i, (x, y, z) in enumerate(verts):
        dx = (x - head_cx) / head_rx
        dy = (y - head_cy) / head_ry
        dz = (z - head_cz) / head_rz
        if dz < 0.3:        # must be on front half
            continue
        if dy > 0.0:        # above equator — not mouth
            continue
        if dy < -0.65:      # too far down (chin tip) — skip
            continue
        dist_from_center = math.sqrt(dx*dx + dz*dz)  # horizontal spread from center
        if dist_from_center > 0.8:
            continue
        # Weight: strongest around dy=-0.3, z=front, x=0
        wy = math.exp(-((dy + 0.3) ** 2) / 0.08)
        wz = max(0.0, (dz - 0.3) / 0.7)
        wx = math.exp(-(dx ** 2) / 0.5)
        w  = wy * wz * wx
        if w > 0.02:
            result.append((i, w))
    return result


def make_morph_deltas(n_verts, mask, viseme):
    """Return list of (dx, dy, dz) per vertex for a given viseme."""
    deltas = [(0.0, 0.0, 0.0)] * n_verts
    for vi, w in mask:
        x, y, z = 0.0, 0.0, 0.0
        vy = 0.0  # normalized dy for this vertex (-1=chin, 0=lip center)
        # Retrieve raw dy (we recalculate inside mask loop for simplicity)
        if   viseme == "viseme_sil":  pass
        elif viseme == "mouthOpen":   y = -w * 0.10
        elif viseme == "viseme_aa":   y = -w * 0.12
        elif viseme == "viseme_O":    z =  w * 0.06;  y = -w * 0.04
        elif viseme == "viseme_E":    x =  w * 0.06;  y = -w * 0.03
        elif viseme == "viseme_I":    x =  w * 0.05;  y = -w * 0.02
        elif viseme == "viseme_U":    z =  w * 0.05;  y = -w * 0.05
        elif viseme == "viseme_PP":   y = -w * 0.01
        elif viseme == "viseme_FF":   y = -w * 0.04;  z =  w * 0.02
        elif viseme == "viseme_TH":   z =  w * 0.04;  y = -w * 0.03
        elif viseme == "viseme_DD":   y = -w * 0.04;  x =  w * 0.02
        elif viseme == "viseme_kk":   y = -w * 0.02
        elif viseme == "viseme_CH":   z =  w * 0.04;  y = -w * 0.02
        elif viseme == "viseme_SS":   y = -w * 0.02;  z =  w * 0.01
        elif viseme == "viseme_nn":   y = -w * 0.01
        elif viseme == "viseme_RR":   y = -w * 0.06;  z =  w * 0.03
        deltas[vi] = (x, y, z)
    return deltas

# ── GLB packing ──────────────────────────────────────────────────────────────

def pack_f32(vals):
    return struct.pack(f'<{len(vals)}f', *vals)

def pack_u16(vals):
    return struct.pack(f'<{len(vals)}H', *vals)

def pack_u32(vals):
    return struct.pack(f'<{len(vals)}I', *vals)

def pad4(data):
    r = len(data) % 4
    return data + b'\x00' * (4 - r) if r else data

def pad4_space(data):
    r = len(data) % 4
    return data + b' ' * (4 - r) if r else data

def bbox(verts):
    xs = [v[0] for v in verts]; ys = [v[1] for v in verts]; zs = [v[2] for v in verts]
    return [min(xs),min(ys),min(zs)], [max(xs),max(ys),max(zs)]


def build_glb(verts, idxs, normals, morph_names, morph_deltas_list, material):
    """
    Build a complete GLB binary.
    morph_deltas_list: list of per-morph list-of-(dx,dy,dz) matching verts length.
    Returns bytes.
    """
    n_verts  = len(verts)
    n_morphs = len(morph_names)

    # ── Binary buffer ────────────────────────────────────────────────────────
    bin_parts = []

    def add_bin(data):
        offset = sum(len(p) for p in bin_parts)
        bin_parts.append(data)
        return offset, len(data)

    # Positions
    pos_flat = [c for v in verts for c in v]
    pos_off, pos_len = add_bin(pack_f32(pos_flat))

    # Normals
    nor_flat = [c for n in normals for c in n]
    nor_off, nor_len = add_bin(pack_f32(nor_flat))

    # Indices (use u32 if needed, else u16)
    max_idx = max(idxs)
    if max_idx < 65535:
        idx_data = pack_u16(idxs)
        idx_type = 5123  # UNSIGNED_SHORT
    else:
        idx_data = pack_u32(idxs)
        idx_type = 5125  # UNSIGNED_INT
    idx_off, idx_len = add_bin(pad4(idx_data))

    # Morph delta buffers (deltas are float32 VEC3, all zeros for non-mouth verts)
    morph_offs = []
    morph_lens = []
    for deltas in morph_deltas_list:
        flat = [c for d in deltas for c in d]
        off, ln = add_bin(pack_f32(flat))
        morph_offs.append(off)
        morph_lens.append(ln)

    bin_blob = b''.join(bin_parts)

    # ── glTF JSON ────────────────────────────────────────────────────────────
    bvs = []  # bufferViews
    acs = []  # accessors

    def add_bv(offset, length, target=None):
        bv = {"buffer": 0, "byteOffset": offset, "byteLength": length}
        if target: bv["target"] = target
        bvs.append(bv)
        return len(bvs) - 1

    def add_ac(bv_idx, count, ctype, etype, mn=None, mx=None):
        ac = {"bufferView": bv_idx, "componentType": ctype, "count": count, "type": etype}
        if mn: ac["min"] = mn
        if mx: ac["max"] = mx
        acs.append(ac)
        return len(acs) - 1

    lo, hi = bbox(verts)
    pos_bv  = add_bv(pos_off, pos_len, 34962)
    pos_ac  = add_ac(pos_bv, n_verts, 5126, "VEC3", lo, hi)

    nor_bv  = add_bv(nor_off, nor_len, 34962)
    nor_ac  = add_ac(nor_bv, n_verts, 5126, "VEC3")

    idx_bv  = add_bv(idx_off, idx_len, 34963)
    idx_ac  = add_ac(idx_bv, len(idxs), idx_type, "SCALAR")

    morph_acs = []
    for i, (off, ln) in enumerate(zip(morph_offs, morph_lens)):
        mv_bv = add_bv(off, ln, 34962)
        # min/max for morph: compute from deltas
        dlts = morph_deltas_list[i]
        dlo = [min(d[c] for d in dlts) for c in range(3)]
        dhi = [max(d[c] for d in dlts) for c in range(3)]
        mv_ac = add_ac(mv_bv, n_verts, 5126, "VEC3", dlo, dhi)
        morph_acs.append(mv_ac)

    targets = [{"POSITION": morph_acs[i]} for i in range(n_morphs)]

    primitive = {
        "attributes": {"POSITION": pos_ac, "NORMAL": nor_ac},
        "indices": idx_ac,
        "material": 0,
        "targets": targets,
        "extras": {"targetNames": morph_names},
    }

    mesh = {"name": "Avatar", "primitives": [primitive],
            "extras": {"targetNames": morph_names}}

    materials = [material]

    gltf = {
        "asset": {"version": "2.0", "generator": "soc-ai-agent-mock avatar gen"},
        "scene": 0,
        "scenes": [{"nodes": [0]}],
        "nodes": [{"mesh": 0, "name": "Avatar"}],
        "meshes": [mesh],
        "materials": materials,
        "accessors": acs,
        "bufferViews": bvs,
        "buffers": [{"byteLength": len(bin_blob)}],
    }

    # ── Assemble GLB ─────────────────────────────────────────────────────────
    json_bytes  = pad4_space(json.dumps(gltf, separators=(',', ':')).encode('utf-8'))
    bin_bytes   = pad4(bin_blob)

    json_chunk  = struct.pack('<II', len(json_bytes), 0x4E4F534A) + json_bytes  # JSON
    bin_chunk   = struct.pack('<II', len(bin_bytes),  0x004E4942) + bin_bytes   # BIN

    body        = json_chunk + bin_chunk
    header      = struct.pack('<4sII', b'glTF', 2, 12 + len(body))
    return header + body


# ── Avatar generation ─────────────────────────────────────────────────────────

def make_avatar(gender):
    # ── Build geometry ───────────────────────────────────────────────────────
    # All in "avatar-local" space. ThreeAvatar normalizes scale automatically.
    # Head sphere centred at (0, 0.9, 0), radius ~0.45
    HEAD_CX, HEAD_CY, HEAD_CZ = 0.0, 0.9, 0.0
    HEAD_RX, HEAD_RY, HEAD_RZ = 0.45, 0.55, 0.45

    head_v, head_i = uv_sphere(HEAD_CX, HEAD_CY, HEAD_CZ,
                                HEAD_RX, HEAD_RY, HEAD_RZ, lat=20, lon=32)

    # Hair — upper cap (lat 0..9) re-used as separate mesh parts
    # We'll just colour the whole head and rely on morph target for lipsync
    # (hair colour is done per-material via a separate primitive later if needed)

    # Neck
    neck_v, neck_i = cylinder(0, 0.3, 0, 0.13, 0.13, 0.3, segs=16,
                               close_top=False, close_bot=False)

    # Torso (suit): slightly tapered cylinder
    torso_r_bot = 0.42 if gender == 'male' else 0.38
    torso_r_top = 0.35 if gender == 'male' else 0.32
    torso_v, torso_i = cylinder(0, -0.6, 0, torso_r_bot, torso_r_top, 0.9,
                                 segs=20, close_top=False, close_bot=True)

    # Shoulder caps (optional — simple half-sphere each side)
    sh_r = 0.14
    sh_y = 0.28
    lsh_v, lsh_i = uv_sphere(-0.44, sh_y, 0.0, sh_r, sh_r, sh_r, lat=8, lon=12)
    rsh_v, rsh_i = uv_sphere( 0.44, sh_y, 0.0, sh_r, sh_r, sh_r, lat=8, lon=12)

    all_verts, all_idxs = merge([
        (head_v,  head_i),
        (neck_v,  neck_i),
        (torso_v, torso_i),
        (lsh_v,   lsh_i),
        (rsh_v,   rsh_i),
    ])

    normals = compute_normals(all_verts, all_idxs)

    # ── Mouth mask on head vertices ──────────────────────────────────────────
    mask = mouth_mask(all_verts, HEAD_CX, HEAD_CY, HEAD_CZ,
                      HEAD_RX, HEAD_RY, HEAD_RZ)

    # ── Morph targets ────────────────────────────────────────────────────────
    morph_names  = ["mouthOpen"] + OCULUS_VISEMES
    morph_deltas = []
    for name in morph_names:
        deltas = make_morph_deltas(len(all_verts), mask, name)
        morph_deltas.append(deltas)

    # ── Material ─────────────────────────────────────────────────────────────
    if gender == 'male':
        skin  = [0.94, 0.82, 0.72, 1.0]
        suit  = [0.18, 0.22, 0.32, 1.0]
    else:
        skin  = [0.96, 0.84, 0.76, 1.0]
        suit  = [0.28, 0.24, 0.38, 1.0]

    material = {
        "name": f"{gender}-avatar",
        "pbrMetallicRoughness": {
            "baseColorFactor": skin,
            "metallicFactor":  0.0,
            "roughnessFactor": 0.85,
        },
        "doubleSided": True,
    }

    return build_glb(all_verts, all_idxs, normals, morph_names, morph_deltas, material)


# ── Main ─────────────────────────────────────────────────────────────────────

if __name__ == '__main__':
    out_dir = os.path.join(os.path.dirname(__file__), '..', 'public', 'avatars')
    os.makedirs(out_dir, exist_ok=True)

    for gender in ('male', 'female'):
        path = os.path.join(out_dir, f'{gender}-avatar.glb')
        glb  = make_avatar(gender)
        with open(path, 'wb') as f:
            f.write(glb)
        size_kb = len(glb) / 1024
        print(f"[OK] {path}  ({size_kb:.0f} KB)")

    print("Done. Both avatars have mouthOpen + 15 Oculus Viseme morph targets.")
