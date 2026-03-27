{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
    nixpkgsUnstable.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flakeUtils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, nixpkgsUnstable, flakeUtils }:
    flakeUtils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        pkgsUnstable = import nixpkgsUnstable {
          inherit system;
          config.allowUnfree = true;
        };
      in {
        packages = flakeUtils.lib.flattenTree {
          go = pkgs.go;
          go-task = pkgsUnstable.go-task;
        };
        devShell = pkgs.mkShell {
          buildInputs = with self.packages.${system}; [
            go
            go-task
          ];
        };
      }
    );
}
