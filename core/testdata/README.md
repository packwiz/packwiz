# Test fixtures

## example-pack/

Verbatim copy of [packwiz/packwiz-example-pack][upstream] at tag `v1`,
minus files irrelevant to the tests (`.gitattributes`, `.gitignore`,
`.packwizignore`, `README.md`).

Used by `example_pack_test.go` for round-trip tests against a real
packwiz pack: `Pack.UpdateIndexHash`, `Index.LoadAllMods`,
`Index.Refresh`.

The vendored pack is **CC0 1.0**, public domain (see
`example-pack/LICENSE`). The upstream README explicitly invites
copying for derivative use.

[upstream]: https://github.com/packwiz/packwiz-example-pack/tree/v1

Do not modify the vendored files — `TestPack_UpdateIndexHash_FromExamplePack`
pins the index hash recorded in the upstream `pack.toml`. Refreshing the
fixture should be done by re-vendoring from upstream rather than
hand-editing.
