let
  # Import nixpkgs if needed
  pkgs = import <nixpkgs> {};
in
  {
    lib ? pkgs.lib,
    buildGoModule ? pkgs.buildGoModule,
    fetchFromGitHub ? pkgs.fetchFromGitHub,
    installShellFiles ? pkgs.installShellFiles,
    # version and vendorHash should be specified by the caller
    version ? "latest",
    vendorHash,
  }:
    buildGoModule rec {
      pname = "packwiz";
      inherit version vendorHash;

      src = ./..;

      nativeBuildInputs = [
        installShellFiles
      ];

      # Install shell completions
      postInstall = ''
        installShellCompletion --cmd packwiz \
          --bash <($out/bin/packwiz completion bash) \
          --fish <($out/bin/packwiz completion fish) \
          --zsh <($out/bin/packwiz completion zsh)
      '';

      meta = with lib; {
        description = "A command line tool for editing and distributing Minecraft modpacks, using a git-friendly TOML format";
        homepage = "https://packwiz.infra.link/";
        license = licenses.mit;
        mainProgram = "packwiz";
      };
    }
