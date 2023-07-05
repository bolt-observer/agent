{
  description = "bolt.observer agent";
  inputs.nixpkgs.url = "nixpkgs/nixos-23.05";

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
            version = "v0.2.1";
            pkgs = nixpkgsFor.${system};
            ldflags = ''-ldflags "-X main.GitRevision=${version} -extldflags '-static'"'';
          in
          {
            bolt-agent = pkgs.buildGoModule
              {
                name = "bolt-agent";
                inherit version;
                src = ./.;
                vendorHash = "sha256-Vy/87OwiDAF4sQ6c8GfTSlYcXvuuF7h4zcjIKNMp5Vo=";
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

      devShells = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [ go gopls gotools go-tools ];
          };
        });

      defaultPackage = forAllSystems (system: self.packages.${system}.bolt-agent);
    };
}
