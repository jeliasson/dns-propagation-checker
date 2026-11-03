{
  description = "Check DNS propagation across many name servers";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      overlays.default = final: _prev: {
        dnspc = final.callPackage ./package.nix { };
      };

      packages = forAllSystems (pkgs: rec {
        dnspc = pkgs.callPackage ./package.nix { };
        default = dnspc;
      });

      # buildGoModule runs go test in its checkPhase, so the package is the check.
      checks = self.packages;

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.gopls ];
        };
      });

      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cli = config.programs.dnspc;
          cfg = config.services.dnspc;
          defaultPackage = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
        in
        {
          options.programs.dnspc = {
            enable = lib.mkEnableOption "the dnspc command line tool";
            package = lib.mkOption {
              type = lib.types.package;
              default = defaultPackage;
              defaultText = lib.literalExpression "dnspc.packages.\${system}.default";
              description = "The dnspc package to install.";
            };
          };

          options.services.dnspc = {
            enable = lib.mkEnableOption "the dnspc web app";

            package = lib.mkOption {
              type = lib.types.package;
              default = defaultPackage;
              defaultText = lib.literalExpression "dnspc.packages.\${system}.default";
              description = "The dnspc package to serve from.";
            };

            address = lib.mkOption {
              type = lib.types.str;
              default = "127.0.0.1";
              example = "0.0.0.0";
              description = ''
                Address to listen on. The default only accepts local
                connections, which is what you want behind a reverse proxy.
              '';
            };

            port = lib.mkOption {
              type = lib.types.port;
              default = 3000;
              description = "Port to listen on.";
            };

            openFirewall = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Whether to open {option}`services.dnspc.port` in the firewall.";
            };

            logLevel = lib.mkOption {
              type = lib.types.enum [ "debug" "info" "warn" "error" ];
              default = "info";
              description = "Log verbosity.";
            };

            trustProxy = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = ''
                Read the client IP from `X-Forwarded-For` so that rate
                limiting sees the real client rather than the proxy. Only
                enable this behind a reverse proxy you control, since
                otherwise any client can spoof the header.
              '';
            };

            extraArgs = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              example = [ "-rate-burst" "60" "-timeout" "5s" ];
              description = "Extra flags passed to {command}`dnspc serve`.";
            };
          };

          config = lib.mkMerge [
            (lib.mkIf cli.enable {
              environment.systemPackages = [ cli.package ];
            })

            (lib.mkIf cfg.enable {
              networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [ cfg.port ];

              systemd.services.dnspc = {
                description = "dnspc DNS propagation checker";
                wantedBy = [ "multi-user.target" ];
                after = [ "network-online.target" ];
                wants = [ "network-online.target" ];

                serviceConfig = {
                  ExecStart = lib.escapeShellArgs ([
                    (lib.getExe cfg.package)
                    "serve"
                    "-addr"
                    "${cfg.address}:${toString cfg.port}"
                    "-log-level"
                    cfg.logLevel
                  ]
                  ++ lib.optional cfg.trustProxy "-trust-proxy"
                  ++ cfg.extraArgs);

                  Restart = "on-failure";
                  RestartSec = "5s";

                  # The binary embeds its frontend and resolves DNS itself, so
                  # it needs nothing from the filesystem beyond its own store
                  # path and nothing from the host resolver.
                  DynamicUser = true;
                  # A privileged port needs the one capability to bind it;
                  # above 1024 the service keeps none at all.
                  AmbientCapabilities = lib.optionalString (cfg.port < 1024) "CAP_NET_BIND_SERVICE";
                  CapabilityBoundingSet = lib.optionalString (cfg.port < 1024) "CAP_NET_BIND_SERVICE";
                  NoNewPrivileges = true;
                  ProtectSystem = "strict";
                  ProtectHome = true;
                  ProtectProc = "invisible";
                  ProcSubset = "pid";
                  PrivateTmp = true;
                  PrivateDevices = true;
                  ProtectClock = true;
                  ProtectHostname = true;
                  ProtectKernelLogs = true;
                  ProtectKernelModules = true;
                  ProtectKernelTunables = true;
                  ProtectControlGroups = true;
                  # INET/INET6 for HTTP and DNS, NETLINK for Go's own resolver
                  # when a name server is given as a hostname.
                  RestrictAddressFamilies = [ "AF_INET" "AF_INET6" "AF_NETLINK" "AF_UNIX" ];
                  RestrictNamespaces = true;
                  RestrictRealtime = true;
                  RestrictSUIDSGID = true;
                  LockPersonality = true;
                  MemoryDenyWriteExecute = true;
                  SystemCallArchitectures = "native";
                  SystemCallFilter = [ "@system-service" "~@privileged" "~@resources" ];
                  UMask = "0077";
                };
              };
            })
          ];
        };
    };
}
