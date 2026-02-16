{
  description = "Punjado - TUI for copying files in a AI friendly format";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable"; # or nixos-24.11
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        # 1. THE PACKAGE BUILD DEFINITION
        packages.default = pkgs.buildGoModule {
          pname = "punjado";
          version = "0.1.0";
          src = ./.;

          # This handles go.sum dependencies.
          # Set to lib.fakeHash initially (see Step 2 below)
          vendorHash = "sha256-fMmO97FfK1dfijI8u1h+UOnnUu8tB1O94S35GQRyt04=";

          # Runtime dependencies (Clipboard support)
          nativeBuildInputs = [ pkgs.makeWrapper ];

          # We wrap the binary so it can find xclip/wl-clipboard
          # even if they aren't installed in your global user profile.
          postInstall = ''
            wrapProgram $out/bin/punjado \
              --prefix PATH : ${
                pkgs.lib.makeBinPath [
                  pkgs.wl-clipboard
                  pkgs.xclip
                ]
              }
          '';
        };

        # 2. YOUR DEV ENVIRONMENT (Kept exactly as you had it)
        devShell = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            delve
            golangci-lint
            xclip
            wl-clipboard
          ];

          shellHook = ''
            echo "🚀 Go TUI Environment Loaded"
          '';
        };
      }
    );
}
