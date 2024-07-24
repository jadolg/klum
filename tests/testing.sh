function assert() {
    if [[ "$1" != "$2" ]]; then
      echo "Test failed: expected '$2', but got '$1'"
      exit 1
    fi
}
