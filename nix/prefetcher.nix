{
  sha256,
  pkgs ? import <nixpkgs> {},
}:
pkgs.callPackage (import ./.) {

  buildGoModule = pkgs.buildGoModule;
    ## As of writing, `pkgs.buildGoModule` is aliased to
    ## `pkgs.buildGo121Module` in Nixpkgs.
    ## `buildGoModule` is set as `pkgs.buildGoModule` to try and work around
    ## `vendorHash` issues in the future.
  vendorHash = sha256;
}
// {
  outputHash = sha256;
}
