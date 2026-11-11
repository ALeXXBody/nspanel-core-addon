#!/bin/bash
echo "Starting to compile MQTTManager."

TARGETPLATFORM=""
STRIP=""
TEST_MODE=0

while true; do
  case "$1" in
  --target-platform)
    export TARGETPLATFORM="$2"
    echo "Will compile for platform $TARGETPLATFORM"
    shift # Remove --target-platform
    shift # Remove value for target platform
    ;;
  --strip)
    echo "Will strip compiled binaries and .so's"
    export STRIP="1"
    shift
    ;;
  --test)
    echo "Will compile for testing. Removing old test DB (if any)."
    rm -f /tmp/nspanelmanager_db_test.sqlite3
    TEST_MODE=1
    shift
    ;;
  *) break ;;
  esac
done

if [ "$TEST_MODE" -eq 1 ]; then
  sed -i 's/set(TEST_MODE.*/set(TEST_MODE 1)/g' /MQTTManager/CMakeLists.txt
  sed -i 's/add_compile_definitions(TEST_MODE.*/add_compile_definitions(TEST_MODE=1)/g' /MQTTManager/CMakeLists.txt
else
  sed -i 's/set(TEST_MODE.*/set(TEST_MODE 0)/g' /MQTTManager/CMakeLists.txt
  sed -i 's/add_compile_definitions(TEST_MODE.*/add_compile_definitions(TEST_MODE=0)/g' /MQTTManager/CMakeLists.txt
fi

if [ -z "$TARGETPLATFORM" ]; then
  TARGETPLATFORM="linux/$(dpkg-architecture -q DEB_BUILD_ARCH_CPU)"
  echo "No platform given as argument, will assume $TARGETPLATFORM"
fi

echo "Compiling for target '$TARGETPLATFORM'"

set -e
set -x

deb_add_arch=""
strip_bin=""
cp /root/.conan2/profiles/default /root/.conan2/profiles/host
if [ "$TARGETPLATFORM" == "linux/386" ]; then
  deb_add_arch="i386"
  conan_target_arch="x86"
  strip_bin="/usr/bin/strip"
  apt -y install gcc-multilib g++-multilib
elif [ "$TARGETPLATFORM" == "linux/amd64" ]; then
  deb_add_arch="amd64"
  conan_target_arch="x86_64"
  strip_bin="/usr/bin/strip"

# Detect the installed versioned cross compiler (Debian ships -12, Ubuntu
# noble ships -13; both provide unversioned symlinks). Prefer the versioned
# binary with the highest version so the conan profile always names a real file.
detect_cross_cc() {
  local prefix="$1"
  local ver
  ver=$(ls /usr/bin/${prefix}-gcc-[0-9]* 2>/dev/null | sort -V | tail -1)
  if [ -n "$ver" ]; then
    basename "$ver"
  else
    echo "${prefix}-gcc"
  fi
}

detect_cross_cxx() {
  local prefix="$1"
  local ver
  ver=$(ls /usr/bin/${prefix}-g++-[0-9]* 2>/dev/null | sort -V | tail -1)
  if [ -n "$ver" ]; then
    basename "$ver"
  else
    echo "${prefix}-g++"
  fi
}

elif [ "$TARGETPLATFORM" == "linux/arm/v6" ]; then
  deb_add_arch="armhf"
  conan_target_arch="armv6"
  strip_bin="/usr/bin/arm-linux-gnueabihf-strip"
  apt -y install binutils-arm-linux-gnueabihf gcc-arm-linux-gnueabihf g++-arm-linux-gnueabihf
  echo "" >>/root/.conan2/profiles/host
  echo "[buildenv]" >>/root/.conan2/profiles/host
  echo "CC=$(detect_cross_cc arm-linux-gnueabihf)" >>/root/.conan2/profiles/host
  echo "CXX=$(detect_cross_cxx arm-linux-gnueabihf)" >>/root/.conan2/profiles/host
  echo "LD=arm-linux-gnueabihf-ld" >>/root/.conan2/profiles/host
elif [ "$TARGETPLATFORM" == "linux/arm/v7" ]; then
  deb_add_arch="armhf"
  conan_target_arch="armv7"
  strip_bin="/usr/bin/arm-linux-gnueabihf-strip"
  apt -y install binutils-arm-linux-gnueabihf gcc-arm-linux-gnueabihf g++-arm-linux-gnueabihf
  echo "" >>/root/.conan2/profiles/host
  echo "[buildenv]" >>/root/.conan2/profiles/host
  echo "CC=$(detect_cross_cc arm-linux-gnueabihf)" >>/root/.conan2/profiles/host
  echo "CXX=$(detect_cross_cxx arm-linux-gnueabihf)" >>/root/.conan2/profiles/host
  echo "LD=arm-linux-gnueabihf-ld" >>/root/.conan2/profiles/host
elif [ "$TARGETPLATFORM" == "linux/arm64" ]; then
  deb_add_arch="arm64"
  conan_target_arch="armv8"
  strip_bin="/usr/bin/aarch64-linux-gnu-strip"
  apt -y install binutils-aarch64-linux-gnu gcc-aarch64-linux-gnu g++-aarch64-linux-gnu
  echo "" >>/root/.conan2/profiles/host
  echo "[buildenv]" >>/root/.conan2/profiles/host
  echo "CC=$(detect_cross_cc aarch64-linux-gnu)" >>/root/.conan2/profiles/host
  echo "CXX=$(detect_cross_cxx aarch64-linux-gnu)" >>/root/.conan2/profiles/host
  echo "LD=aarch64-linux-gnu-ld" >>/root/.conan2/profiles/host
  #echo "CFLAGS=-march=armv8-a" >>/root/.conan2/profiles/host
  #echo "CXXFLAGS=-march=armv8-a" >>/root/.conan2/profiles/host
else
  echo "ERROR! Unknown target platform. Will exit."
  exit 1
fi
eval $(dpkg-architecture) # Load in what the builder machine is

# Setup apt source file to be able to access both architectures
# Works on Debian (debian.sources deb822), Ubuntu (one-line arm64 entry to
# ubuntu-ports) and classic sources.list layouts.
if [ "$deb_add_arch" != "$DEB_HOST_ARCH" ]; then
  if [ -f /etc/apt/sources.list.d/debian.sources ]; then
    # Debian 12+ layout (the Docker build environment)
    if [ ! -f "/etc/apt/sources.list.d/debian.sources.bak" ]; then
      cp /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list.d/debian.sources.bak
    fi
    cp /etc/apt/sources.list.d/debian.sources.bak /etc/apt/sources.list.d/debian.sources
    dpkg --add-architecture $deb_add_arch
    sed -i "/Components.*/a arch=$DEB_HOST_ARCH,$deb_add_arch" /etc/apt/sources.list.d/debian.sources
    apt-get update
  elif [ -f /etc/apt/sources.list.d/ubuntu.sources ]; then
    # Ubuntu (CI runners): existing deb822 stanzas must not be modified with
    # arch= options. Restrict them to amd64 and add a dedicated one-line
    # arm64 entry against ubuntu-ports (which carries arm64 packages).
    CODENAME=$(. /etc/os-release && echo "${VERSION_CODENAME:-noble}")
    sed -i 's/^Types: deb$/Types: deb\nArchitectures: amd64/' /etc/apt/sources.list.d/ubuntu.sources
    cat > /etc/apt/sources.list.d/nspanel-cross.list <<EOFL
deb [arch=$deb_add_arch] http://ports.ubuntu.com/ubuntu-ports $CODENAME main restricted universe multiverse
deb [arch=$deb_add_arch] http://ports.ubuntu.com/ubuntu-ports ${CODENAME}-updates main restricted universe multiverse
deb [arch=$deb_add_arch] http://ports.ubuntu.com/ubuntu-ports ${CODENAME}-backports main restricted universe multiverse
deb [arch=$deb_add_arch] http://ports.ubuntu.com/ubuntu-ports ${CODENAME}-security main restricted universe multiverse
EOFL
    dpkg --add-architecture $deb_add_arch
    apt-get update
  else
    echo "ERROR: No known apt sources layout found for cross-architecture setup."
    exit 1
  fi
fi

apt -y install libssl-dev:$deb_add_arch openssl:$deb_add_arch libcurl4-openssl-dev:$deb_add_arch

BASEDIR=$(dirname "$0")
pushd "$BASEDIR"
cd /MQTTManager/

# rm -rf build
if [ -e CMakeCache.txt ]; then
	rm CMakeCache.txt
	rm -rf CMakeFiles
fi

#sed -i "s/^arch.*/arch=$build_arch/g" /root/.conan2/profiles/host
# Get build type from conan profile
BUILD_TYPE=$(grep -E "^build_type=" /root/.conan2/profiles/default | cut -d'=' -f 2)
sed -i "s/arch=.*/arch=${conan_target_arch}/g" /root/.conan2/profiles/host

# Determine wether to link libraries statically or dynamically.
if [ "$BUILD_TYPE" == "Debug" ]; then
  BUILD_SHARED_LIBS=ON
else
  BUILD_SHARED_LIBS=OFF
fi

echo "Conan profile: "
cat /root/.conan2/profiles/default

echo "Will build MQTTManager in ${BUILD_TYPE} mode."

conan install . --build=missing -pr:b default -pr:h host
echo "--> Conan install complete."
cd build
source $BUILD_TYPE/generators/conanbuild.sh
cmake .. -DCMAKE_TOOLCHAIN_FILE=$BUILD_TYPE/generators/conan_toolchain.cmake -DCMAKE_BUILD_TYPE=$BUILD_TYPE -DBUILD_SHARED_LIBS=${BUILD_SHARED_LIBS}
cmake --build . --config $BUILD_TYPE -j $(nproc)
if [ -f /MQTTManager/fix_compile_commands_path.sh ]; then
  source /MQTTManager/fix_compile_commands_path.sh
fi
cp compile_commands.json ../

if [ "$STRIP" == "1" ]; then
  echo "Stripping binaries and .so files using '$strip_bin'..."
  if compgen -G '/MQTTManager/build/*.so' > /dev/null; then
    "$strip_bin" /MQTTManager/build/*.so
  fi
  "$strip_bin" /MQTTManager/build/nspm_mqttmanager
fi
