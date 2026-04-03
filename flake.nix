{
  description = "A flake enviroment for better devex";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        commonPackages = with pkgs; [
          go
          bashInteractive
          coreutils
          curl
          findutils
          gawk
          git
          gnugrep
		  go
          gnumake
          jq
          sqlc
          unzip
          zip
          docker-client
          docker-compose
        ];
      in
      {
        devShells.default = pkgs.mkShell {
          packages = commonPackages;

          shellHook = '''';
        };

        formatter = pkgs.nixpkgs-fmt;

        apps = {
          shell = {
            type = "app";
            program = "${pkgs.bash}/bin/bash";
          };
        };
      });
}
