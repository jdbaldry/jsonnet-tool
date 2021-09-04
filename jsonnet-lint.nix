{ pkgs ? import <nixpkgs> }:

with pkgs;
buildGoModule rec {
  pname = "jsonnet-lint";
  version = "0.17.0";

  doCheck = false;
  src = fetchFromGitHub {
    owner = "google";
    repo = "go-jsonnet";
    rev = "v${version}";
    sha256 = "sha256-KLHtPR8hkKof5p/QC5CEXwhaWECvXcIb6XnZEijS+eY=";
  };
  subPackages = [ "cmd/jsonnet-lint" ];
  vendorSha256 = "sha256-wUoR67BtUgb0jChkvapq8Mc4GD7VROfJ9xXZtfQjVVs=";

  meta = with lib; {
    description = "Jsonnet linter";
    homepage = "https://github.com/google/go-jsonnet/tree/master/cmd/jsonnet-lint";
    license = licenses.asl20;
    maintainers = with maintainers; [ jdbaldry ];
  };
}
