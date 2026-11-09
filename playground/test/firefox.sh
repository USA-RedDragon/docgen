#!/bin/sh
# firefox.sh PROFILE PORT LOG starts headless Firefox with a new profile in the
# directory PROFILE and Marionette on 127.0.0.1:PORT, and prints its PID. The
# console messages of pages go to the file LOG.
set -eu
prof=$1
port=$2
log=$3
rm -rf "$prof"
mkdir -p "$prof"
cat >"$prof/user.js" <<EOP
user_pref("marionette.port", $port);
user_pref("browser.shell.checkDefaultBrowser", false);
user_pref("datareporting.policy.dataSubmissionEnabled", false);
user_pref("toolkit.telemetry.reportingpolicy.firstRun", false);
user_pref("browser.startup.homepage_override.mstone", "ignore");
user_pref("app.update.disabledForTesting", true);
user_pref("browser.cache.disk.enable", false);
user_pref("devtools.console.stdout.content", true);
user_pref("dom.cache.enabled", true);
user_pref("layout.css.devPixelsPerPx", "-1.0");
user_pref("layout.css.prefers-color-scheme.content-override", 2);
user_pref("dom.events.testing.asyncClipboard", true);
EOP
MOZ_REMOTE_ALLOW_SYSTEM_ACCESS=1 firefox --headless --marionette --no-remote -profile "$prof" >"$log" 2>&1 &
pid=$!
i=0
while [ ! -s "$prof/MarionetteActivePort" ]; do
	i=$((i + 1))
	if [ "$i" -gt 120 ] || ! kill -0 "$pid" 2>/dev/null; then
		echo "firefox.sh: Marionette did not start; see $log" >&2
		kill "$pid" 2>/dev/null || :
		exit 1
	fi
	sleep 0.5
done
echo "$pid"
