function assert() {
    if [[ "$1" != "$2" ]]; then
      echo "Test failed: expected '$2', but got '$1'"
      exit 1
    fi
}

function assert_fail() {
    if "$@"; then
        echo "Test failed: command succeeded, but it was expected to fail"
        exit 1
    else
        echo "Test passed: command failed as expected"
    fi
}

function eventually() {
    max_attempts=20
    for ((i=1; i<=max_attempts; i++)); do
      if "$@"; then
        echo "Command succeeded"
        return 0
      else
        echo "Attempt $i failed"
      fi
      sleep 5
    done
    echo "Command failed after $max_attempts attempts"
    exit 1
}
