{
  sha256,
  pkgs ? import <nixpkgs> {},
}:
pkgs.callPackage (import ./.) {
  ## Keeping `buildGoModule` commented out in case it's needed in the future.
  ## As of writing, `buildGo121Module` is currently used as the default for
  ## `buildGoModule` in nixpkgs. `buildGo118Module` seems deprecated and
  ## entirely removed from nixpkgs, so manually setting that is likely to cause
  ## issues in the future.

  # buildGoModule = pkgs.buildGo118Module;
  vendorSha256 = sha256;
}
// {
  outputHash = sha256;
}
