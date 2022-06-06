{
  inputs.nixpkgs.url = "nixpkgs/nixos-unstable";

  outputs = {
    self,
    nixpkgs,
  }:
    with nixpkgs.lib; let
      # Compute version string-friendly date from last commit date.
      lastModifiedPretty = let
        year = substring 0 4 self.lastModifiedDate;
        month = substring 4 2 self.lastModifiedDate;
        day = substring 6 2 self.lastModifiedDate;
      in "${year}-${month}-${day}";

      # Supported systems.
      # TODO: Whih systems does packwiz actually support ?
      supportedSystems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];

      # Helper generating outputs for each supported system
      forAllSystems = genAttrs supportedSystems;

      # Import nixpkgs' package set for each system.
      nixpkgsFor = forAllSystems (system: import nixpkgs {inherit system;});
    in {
      # Packwiz package
      packages = forAllSystems (system: let
        pkgs = nixpkgsFor.${system};
      in rec {
        packwiz = pkgs.callPackage ./default.nix {
          version = "nightly-${lastModifiedPretty}";
          vendorSha256 = readFile ./vendor-sha256;
          buildGoModule = pkgs.buildGo118Module;
        };
        # Build packwiz by default when no package name is specified
        default = packwiz;
      });

      # This flake's nix code formatter
      formatter = forAllSystems (system: nixpkgsFor.${system}.alejandra);
    };
}
