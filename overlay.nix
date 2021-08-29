final: prev:

rec {
  jsonnet-tool = prev.callPackage ./. { pkgs = prev; };
}
