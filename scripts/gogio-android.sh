#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 5 ]; then
	echo "usage: $0 <gogio-bin> <min-sdk> <target-sdk> <app-id> <go-package>" >&2
	exit 2
fi

gogio_bin="$1"
min_sdk="$2"
target_sdk="$3"
app_id="$4"
go_pkg="$5"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

if ! command -v "$gogio_bin" >/dev/null 2>&1; then
	echo "gogio binary not found: $gogio_bin" >&2
	exit 1
fi

android_home="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
if [ -z "$android_home" ] || [ ! -d "$android_home" ]; then
	echo "ANDROID_HOME/ANDROID_SDK_ROOT is not set to a valid Android SDK path" >&2
	exit 1
fi

build_tools_dir="$(find "$android_home/build-tools" -mindepth 1 -maxdepth 1 -type d | sort | tail -n1 || true)"
platform_dir="$(find "$android_home/platforms" -mindepth 1 -maxdepth 1 -type d -name 'android-*' | sort -V | tail -n1 || true)"
if [ -z "$build_tools_dir" ] || [ -z "$platform_dir" ]; then
	echo "Android build-tools or platforms are missing under $android_home" >&2
	exit 1
fi

aapt2_bin="$build_tools_dir/aapt2"
zipalign_bin="$build_tools_dir/zipalign"
apksigner_bin="$build_tools_dir/apksigner"
android_jar="$platform_dir/android.jar"

for tool in "$aapt2_bin" "$zipalign_bin" "$apksigner_bin"; do
	if [ ! -x "$tool" ]; then
		echo "required Android tool is missing: $tool" >&2
		exit 1
	fi
done

log_file="$(mktemp /tmp/gogio-build-XXXX.log)"
"$gogio_bin" -x -work -target android -minsdk "$min_sdk" -targetsdk "$target_sdk" -appid "$app_id" "$go_pkg" >"$log_file" 2>&1

workdir="$(sed -n 's/^WORKDIR=//p' "$log_file" | tail -n1)"
if [ -z "$workdir" ] || [ ! -d "$workdir" ]; then
	echo "cannot determine gogio WORKDIR from log" >&2
	sed -n '1,200p' "$log_file" >&2
	exit 1
fi

manifest="$workdir/AndroidManifest.xml"
if [ ! -f "$manifest" ]; then
	echo "manifest not found in WORKDIR: $manifest" >&2
	exit 1
fi

notif_icon_dir="$workdir/res/drawable"
notif_icon_file="$notif_icon_dir/ic_stat_wetterabhaengig.xml"
mkdir -p "$notif_icon_dir"
cat >"$notif_icon_file" <<'EOF'
<?xml version="1.0" encoding="utf-8"?>
<vector xmlns:android="http://schemas.android.com/apk/res/android"
	android:width="24dp"
	android:height="24dp"
	android:viewportWidth="24"
	android:viewportHeight="24">
	<path
		android:fillColor="#00000000"
		android:strokeColor="#FFFFFFFF"
		android:strokeLineJoin="round"
		android:strokeLineCap="round"
		android:strokeWidth="1.6"
		android:pathData="M8,2.5h8c1.1,0 2,0.9 2,2v15c0,1.1 -0.9,2 -2,2H8c-1.1,0 -2,-0.9 -2,-2v-15c0,-1.1 0.9,-2 2,-2z"/>
	<path
		android:fillColor="#FFFFFFFF"
		android:pathData="M12,7m-1.6,0a1.6,1.6 0,1 0,3.2 0a1.6,1.6 0,1 0,-3.2 0"/>
	<path
		android:fillColor="#FFFFFFFF"
		android:pathData="M12,12m-1.6,0a1.6,1.6 0,1 0,3.2 0a1.6,1.6 0,1 0,-3.2 0"/>
	<path
		android:fillColor="#FFFFFFFF"
		android:pathData="M12,17m-1.6,0a1.6,1.6 0,1 0,3.2 0a1.6,1.6 0,1 0,-3.2 0"/>
</vector>
EOF

if ! grep -q 'android.permission.POST_NOTIFICATIONS' "$manifest"; then
	tmp_manifest="$(mktemp /tmp/AndroidManifest-XXXX.xml)"
	awk '
		/<application / {
			print "\t<uses-permission android:name=\"android.permission.POST_NOTIFICATIONS\"/>"
			added=1
		}
		{ print }
		END {
			if (!added) {
				print "\t<uses-permission android:name=\"android.permission.POST_NOTIFICATIONS\"/>"
			}
		}
	' "$manifest" >"$tmp_manifest"
	mv "$tmp_manifest" "$manifest"
fi

"$aapt2_bin" compile -o "$workdir/resources.zip" --dir "$workdir/res"
"$aapt2_bin" link --manifest "$manifest" -I "$android_jar" -o "$workdir/link.apk" "$workdir/resources.zip"

repack_dir="$workdir/repack"
rm -rf "$repack_dir"
mkdir -p "$repack_dir"
unzip -q "$workdir/link.apk" -d "$repack_dir"

for arch_dir in "$workdir"/jni/*; do
	arch_name="$(basename "$arch_dir")"
	mkdir -p "$repack_dir/lib/$arch_name"
	cp "$arch_dir/libgio.so" "$repack_dir/lib/$arch_name/libgio.so"
done

cp "$workdir/apk/classes.dex" "$repack_dir/classes.dex"

(
	cd "$repack_dir"
	rm -f "$workdir/app.zip"
	zip -q0r "$workdir/app.zip" .
)

output_apk="wetterabhaengig.apk"
"$zipalign_bin" -f 4 "$workdir/app.zip" "$output_apk"

keystore_path="${WETTER_KEYSTORE_PATH:-$repo_root/.keys/android-debug.keystore}"
keystore_alias="${WETTER_KEY_ALIAS:-android}"
keystore_pass="${WETTER_KEY_PASS:-android}"

mkdir -p "$(dirname "$keystore_path")"
if [ ! -f "$keystore_path" ]; then
	keytool -genkey -keystore "$keystore_path" -storepass "$keystore_pass" -alias "$keystore_alias" -keyalg RSA -keysize 2048 -validity 10000 -noprompt -dname CN=wetterabhaengig
fi

"$apksigner_bin" sign --ks-pass "pass:$keystore_pass" --ks "$keystore_path" --ks-key-alias "$keystore_alias" "$output_apk"

echo "Patched APK built at: $(pwd)/$output_apk"
