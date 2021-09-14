{ pkgs ? import <nixpkgs> }:
with pkgs;
mkShell {
  buildInputs = [
    gofumpt
    go-outline
    go-tools
    go_1_16
    goimports
    golangci-lint
    gopkgs
    gopls
    rr
  ] ++ [ feh graphviz mupdf rlwrap ] ++ [ jsonnet-lint ];
  shellHook = ''
    # ...
  '';
}
