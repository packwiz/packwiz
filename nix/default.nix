{
  lib ? lib,
  buildGoModule ? buildGoModule,
  installShellFiles ? installShellFiles,
  # version should be specified by the caller
  version ? "latest",
}:
buildGoModule {
  pname = "packwiz";
  inherit version;

  src = ./..;

  vendorHash = "sha256-ChUE4hWl+UyPpbzK0GbJTD0AoBCogI7qGstga4+WujI=";

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
