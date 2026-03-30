{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flakeUtils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, flakeUtils }:
    flakeUtils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages = flakeUtils.lib.flattenTree {
          go = pkgs.go;
          make = pkgs.gnumake;
        };
        devShell = pkgs.mkShell {
          buildInputs = with self.packages.${system}; [
            go
            make
          ];
          shellHook = ''
            export GOBIN="$PWD/.bin"
            export PATH="$GOBIN:$PATH"
          '';
        };
      }
    );
}
