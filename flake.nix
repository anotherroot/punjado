{
  description = "Punjado - TUI for copying files in a AI friednly format";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-25.11";
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
        pkgs = import nixpkgs {
          inherit system;
        };
      in
      {
        devShell = pkgs.mkShell {
          buildInputs = with pkgs; [
            # 1. Core Go Tools
            go # The Go compiler (defaults to latest in 25.11)
            gopls # The official Go Language Server (Crucial for VS Code/Helix autocompletion)
            delve # The Go debugger (dlv)
            golangci-lint # The standard linter

            # 2. System Dependencies for Clipboard (Crucial for your specific project)
            # bubbletea/clipboard needs these to talk to the Linux system clipboard
            xclip # For X11 clipboard support
            wl-clipboard # For Wayland clipboard support
          ];

          shellHook = ''
            echo "🚀 Go TUI Environment Loaded"
            echo "   Go Version: $(go version)"
            echo "   Gopls Version: $(gopls version | head -n 1)"
          '';
        };
      }
    );
}
