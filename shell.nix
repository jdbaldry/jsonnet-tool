{ pkgs ? import <nixpkgs> }:
with pkgs;
mkShell {
  buildInputs = [ go_1_16 gopls ] ++ [ feh graphviz mupdf ] ++ [ jsonnet-lint ];
  shellHook = ''
    # ...
  '';
}
