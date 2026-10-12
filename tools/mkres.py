#!/usr/bin/env python3
"""Build rsrc_windows_amd64.syso (exe icon, manifest, version info) with only the
Python standard library. `go build` links any .syso in the package automatically.

Usage: python3 tools/mkres.py <app.ico> <manifest.xml> <version> <out.syso>
"""
import struct
import sys

RT_ICON, RT_GROUP_ICON, RT_VERSION, RT_MANIFEST = 3, 14, 16, 24
LANG = 0x0409


def align(b, n=4):
    return b + b"\0" * (-len(b) % n)


def wstr(s):
    return (s + "\0").encode("utf-16-le")


# ---------- icon ----------
def read_ico(path):
    data = open(path, "rb").read()
    _, typ, count = struct.unpack_from("<HHH", data, 0)
    assert typ == 1, "not an .ico file"
    entries = []
    for i in range(count):
        w, h, cc, res, planes, bpp, size, off = struct.unpack_from("<BBBBHHII", data, 6 + 16 * i)
        entries.append(((w, h, cc, res, planes, bpp, size), data[off:off + size]))
    return entries


def group_icon(entries):
    out = struct.pack("<HHH", 0, 1, len(entries))
    for i, ((w, h, cc, res, planes, bpp, size), _) in enumerate(entries):
        out += struct.pack("<BBBBHHIH", w, h, cc, res, planes or 1, bpp or 32, size, i + 1)
    return out


# ---------- version info ----------
def vs_block(key, value=b"", value_len=0, typ=0, children=b""):
    body = align(struct.pack("<HHH", 0, value_len, typ) + wstr(key))
    body += align(value) + children
    return struct.pack("<H", len(body)) + body[2:]


def version_info(version, strings):
    parts = [int(x) for x in (version.split(".") + ["0"] * 4)[:4]]
    ms, ls = (parts[0] << 16) | parts[1], (parts[2] << 16) | parts[3]
    fixed = struct.pack("<13I", 0xFEEF04BD, 0x00010000, ms, ls, ms, ls, 0x3F, 0, 0x00040004, 1, 0, 0, 0)
    table = b""
    for k, v in strings.items():
        val = wstr(v)
        table += align(vs_block(k, val, len(val) // 2, 1))
    sfi = vs_block("StringFileInfo", typ=1, children=align(vs_block("040904B0", typ=1, children=table)))
    var = vs_block("Translation", struct.pack("<HH", LANG, 1200), 4, 0)
    vfi = vs_block("VarFileInfo", typ=1, children=align(var))
    return vs_block("VS_VERSION_INFO", fixed, len(fixed), 0, align(sfi) + align(vfi))


# ---------- resource tree -> COFF ----------
def build_rsrc(resources):
    """resources: {type: {id: bytes}} (all numeric ids, single language)."""
    types = sorted(resources)
    # layout: root dir, type dirs, name dirs, data entries, data blobs
    dir_size = lambda n: 16 + 8 * n
    off = dir_size(len(types))
    type_off, name_off = {}, {}
    for t in types:
        type_off[t] = off
        off += dir_size(len(resources[t]))
    for t in types:
        for i in sorted(resources[t]):
            name_off[(t, i)] = off
            off += dir_size(1)
    entry_off = {}
    for t in types:
        for i in sorted(resources[t]):
            entry_off[(t, i)] = off
            off += 16
    data_off = {}
    for t in types:
        for i in sorted(resources[t]):
            off += -off % 8
            data_off[(t, i)] = off
            off += len(resources[t][i])

    buf = bytearray(off)
    relocs = []

    def put_dir(at, ids_and_targets):
        struct.pack_into("<IIHHHH", buf, at, 0, 0, 0, 0, 0, len(ids_and_targets))
        for n, (ident, target, is_dir) in enumerate(ids_and_targets):
            struct.pack_into("<II", buf, at + 16 + 8 * n, ident, target | (0x80000000 if is_dir else 0))

    put_dir(0, [(t, type_off[t], True) for t in types])
    for t in types:
        put_dir(type_off[t], [(i, name_off[(t, i)], True) for i in sorted(resources[t])])
        for i in sorted(resources[t]):
            put_dir(name_off[(t, i)], [(LANG, entry_off[(t, i)], False)])
            e = entry_off[(t, i)]
            struct.pack_into("<IIII", buf, e, data_off[(t, i)], len(resources[t][i]), 0, 0)
            relocs.append(e)  # OffsetToData is an RVA -> needs ADDR32NB relocation
            d = data_off[(t, i)]
            buf[d:d + len(resources[t][i])] = resources[t][i]

    raw = bytes(buf)
    hdr_size, sec_size = 20, 40
    raw_ptr = hdr_size + sec_size
    reloc_ptr = raw_ptr + len(raw)
    sym_ptr = reloc_ptr + 10 * len(relocs)
    out = struct.pack("<HHIIIHH", 0x8664, 1, 0, sym_ptr, 1, 0, 0)
    out += struct.pack("<8sIIIIIIHHI", b".rsrc", 0, 0, len(raw), raw_ptr, reloc_ptr, 0, len(relocs), 0, 0x40000040)
    out += raw
    for r in relocs:
        out += struct.pack("<IIH", r, 0, 3)  # IMAGE_REL_AMD64_ADDR32NB against symbol 0 (.rsrc)
    out += struct.pack("<8sIhHBB", b".rsrc", 0, 1, 0, 3, 0)
    out += struct.pack("<I", 4)  # empty string table
    return out


def main():
    ico, manifest, version, out = sys.argv[1:5]
    entries = read_ico(ico)
    res = {
        RT_ICON: {i + 1: blob for i, (_, blob) in enumerate(entries)},
        RT_GROUP_ICON: {1: group_icon(entries)},
        RT_MANIFEST: {1: open(manifest, "rb").read()},
        RT_VERSION: {1: version_info(version, {
            "CompanyName": "mimikm",
            "FileDescription": "Audio Switcher",
            "FileVersion": version,
            "InternalName": "AudioSwitcher",
            "LegalCopyright": "MIT License",
            "OriginalFilename": "AudioSwitcher.exe",
            "ProductName": "Audio Switcher",
            "ProductVersion": version,
        })},
    }
    open(out, "wb").write(build_rsrc(res))


if __name__ == "__main__":
    main()
