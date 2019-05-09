#!/bin/bash

#ERR pseudo-signal is only supported by bash.

# error handling with trap taken from https://unix.stackexchange.com/questions/79648/how-to-trigger-error-using-trap-command/157327
unset killer_sig 
for sig in SIGHUP SIGINT SIGQUIT SIGTERM; do
  trap '
    killer_sig="$sig"
    exit $((128 + $(kill -l "$sig")))' "$sig"
done

trap '
  ret=$?
  [ "$ret" -eq 0 ] || echo >&2 "Terminating with error!"
  if [ -n "$killer_sig" ]; then
    trap - "$killer_sig" # reset traps
    kill -s "$killer_sig" "$$"
  else
    exit "$ret"
  fi' EXIT

trap '
  ret=$?
  error_handler $ret $LINENO $BASH_COMMAND' ERR

error_handler()
{
  echo "$(basename $0) error on line : $2 command was: $3"
  exit $1
}

if [[ ( "$TRAVIS_PIPELINE" = "buildah" ) || ( "$TRAVIS_PIPELINE" = ""  && -x "$(command -v buildah)" ) ]]; then
  source ./build_buildah.sh
elif [[ ( "$TRAVIS_PIPELINE" = "docker" ) || ( "$TRAVIS_PIPELINE" = ""  && -x "$(command -v docker)" ) ]]; then
  source ./build_docker.sh
else
 echo 'The build process requires docker or buildah/podman installed. Please install any of these and make sure these are executable'
 exit 1
fi

