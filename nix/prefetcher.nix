{
  sha256,
  pkgs ? import <nixpkgs> {},
}:
pkgs.callPackage (import ./.) {
  buildGoModule = pkgs.buildGo118Module;
  vendorSha256 = sha256;
}
// {
  outputHash = sha256;
}
