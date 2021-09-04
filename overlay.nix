final: prev:

rec {
  jsonnet-lint = prev.callPackage ./jsonnet-lint.nix { pkgs = prev; };
  jsonnet-tool = prev.callPackage ./. { pkgs = prev; };
}
