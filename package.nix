# dnspc - check DNS propagation across many name servers, from the terminal
# or as a small web app. Pure Go with the frontend embedded in the binary,
# so the package has no runtime dependencies at all.

{ lib
, buildGoModule
}:

buildGoModule rec {
  pname = "dnspc";
  version = "0.1.0";

  src = lib.cleanSource ./.;
  vendorHash = "sha256-rIX2NB3TgEakHte6hzWHAAjeMhTjcyCel+ZfDFEf5Ck=";

  ldflags = [
    "-s"
    "-w"
    "-X github.com/jeliasson/dns-propagation-checker/internal/cli.Version=${version}"
  ];

  meta = {
    description = "Check DNS propagation across many name servers";
    homepage = "https://github.com/jeliasson/dns-propagation-checker";
    license = lib.licenses.mit;
    mainProgram = "dnspc";
    platforms = lib.platforms.unix;
  };
}
