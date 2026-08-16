#!/bin/bash

set -e

apt_get_update()
{
    if [ "$(find /var/lib/apt/lists/* | wc -l)" = "0" ]; then
        echo "Running apt-get update..."
        apt-get update -y
    fi
}

# Checks if packages are installed and installs them if not
check_packages() {
    if ! dpkg -s "$@" > /dev/null 2>&1; then
        apt_get_update
        apt-get -y install --no-install-recommends "$@"
    fi
}

check_packages curl jq ca-certificates

VERSION=${VERSION:-latest}

if [[ "$VERSION" = latest ]]; then
    VERSION="$(curl -fsSL https://api.github.com/repos/sivchari/kumo/releases/latest | jq -r .tag_name)"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

case "$(uname -m)" in
    x86_64) arch=amd64;;
    aarch64) arch=arm64;;
    *) echo "Not supported" >&2; exit 255;;
esac

url="https://github.com/sivchari/kumo/releases/download/$VERSION/kumo_${VERSION#v}_linux_$arch.tar.gz"
curl -L "$url" -o "$tmp/kumo.tar.gz"
tar -xf "$tmp/kumo.tar.gz" -C /usr/local/bin kumo

tee /usr/local/share/kumo-init.sh << 'EOF'
#!/bin/bash
set -e

/usr/local/bin/kumo > /var/log/kumo.log 2>&1 &

exec "$@"
EOF
chmod +x /usr/local/share/kumo-init.sh
