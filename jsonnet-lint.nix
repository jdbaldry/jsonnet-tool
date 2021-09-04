{ pkgs ? import <nixpkgs> }:

with pkgs;
buildGoModule rec {
  pname = "jsonnet-lint";
  version = "0.17.0";

  src = fetchFromGitHub {
    owner = "google";
    repo = "go-jsonnet";
    rev = "v${version}";
    sha256 = lib.fakeSha256;
  };
  vendorSha256 = lib.fakeSha256;

  meta = with lib; {
    description = "Jsonnet linter";
    homepage = "https://github.com/google/go-jsonnet/tree/master/cmd/jsonnet-lint";
    license = license.asl20;
    maintainers = with maintainers; [ jdbaldry ];
  };
}
