{
  description = "bolt.observer agent";
  inputs.nixpkgs.url = "nixpkgs/nixos-22.11";

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
    in
    {
      packages = forAllSystems
        (system:
          let
            version = "v0.0.48";
            pkgs = nixpkgsFor.${system};
            ldflags = ''-ldflags "-X main.GitRevision=${version} -extldflags '-static'"'';
          in
          {
            bolt-agent = pkgs.buildGoModule
              {
                name = "bolt-agent";
                inherit version;
                src = ./.;
                vendorHash = "sha256-W4OuqLvW5gdbsEQRmsAdVXaLbrT9KeqRDyCtnX77ipI=";
                doCheck = false;
                doInstallCheck = false;

                meta = with pkgs.lib; {
                  description = "bolt.observer agent";
                };

                preBuild = ''
                  buildFlagsArray+=(${ldflags})
                  buildFlagsArray+=("-tags=timetzdata,plugins")
                '';
              };
          });

      # Add dependencies that are only needed for development
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [ go gopls gotools go-tools ];
          };
        });

      # The default package for 'nix build'. This makes sense if the
      # flake provides only one package or there is a clear "main"
      # package.
      defaultPackage = forAllSystems (system: self.packages.${system}.bolt-agent);
    };
}
