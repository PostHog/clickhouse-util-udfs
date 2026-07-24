#!/usr/bin/env bash

UDFS=(
  json_remove_empty_strings
  json_remove_duplicate_keys
  json_drop_keys
  json_clean_posthog_event_properties
  decompress
)

normalize_udf() {
  case "$1" in
    json_remove_empty_strings|json_remove_empty_strings_udf|JSONRemoveEmptyStrings)
      echo "json_remove_empty_strings"
      ;;
    json_remove_duplicate_keys|json_key_dedup|json_key_dedup_udf|JSONRemoveDuplicateKeys)
      echo "json_remove_duplicate_keys"
      ;;
    json_drop_keys|json_drop_keys_udf|JSONDropKeys)
      echo "json_drop_keys"
      ;;
    json_clean_posthog_event_properties|json_clean_posthog_event_properties_udf|JSONCleanPostHogEventProperties)
      echo "json_clean_posthog_event_properties"
      ;;
    decompress|decompress_udf)
      echo "decompress"
      ;;
    *)
      echo "Unknown UDF: $1" >&2
      return 1
      ;;
  esac
}

resolve_udfs() {
  if [[ $# -eq 0 ]]; then
    printf '%s\n' "${UDFS[@]}"
    return
  fi

  local udf
  for udf in "$@"; do
    if [[ "$udf" == "all" ]]; then
      printf '%s\n' "${UDFS[@]}"
    else
      normalize_udf "$udf"
    fi
  done
}

udf_binary() {
  case "$1" in
    json_remove_empty_strings)
      echo "json_remove_empty_strings_udf"
      ;;
    json_remove_duplicate_keys)
      echo "json_key_dedup_udf"
      ;;
    json_drop_keys)
      echo "json_drop_keys_udf"
      ;;
    json_clean_posthog_event_properties)
      echo "json_clean_posthog_event_properties_udf"
      ;;
    decompress)
      echo "decompress_udf"
      ;;
    *)
      echo "Unknown UDF: $1" >&2
      return 1
      ;;
  esac
}

udf_function() {
  case "$1" in
    json_remove_empty_strings)
      echo "JSONRemoveEmptyStrings"
      ;;
    json_remove_duplicate_keys)
      echo "JSONRemoveDuplicateKeys"
      ;;
    json_drop_keys)
      echo "JSONDropKeys"
      ;;
    json_clean_posthog_event_properties)
      echo "JSONCleanPostHogEventProperties"
      ;;
    decompress)
      echo "decompress"
      ;;
    *)
      echo "Unknown UDF: $1" >&2
      return 1
      ;;
  esac
}

udf_xml_file() {
  case "$1" in
    json_remove_empty_strings)
      echo "JSONRemoveEmptyStrings_function.xml"
      ;;
    json_remove_duplicate_keys)
      echo "JSONRemoveDuplicateKeys_function.xml"
      ;;
    json_drop_keys)
      echo "JSONDropKeys_function.xml"
      ;;
    json_clean_posthog_event_properties)
      echo "JSONCleanPostHogEventProperties_function.xml"
      ;;
    decompress)
      echo "decompress_function.xml"
      ;;
    *)
      echo "Unknown UDF: $1" >&2
      return 1
      ;;
  esac
}

udf_test_query() {
  local input_file=${2:-input.tsv}

  case "$1" in
    json_remove_empty_strings)
      echo "SELECT JSONRemoveEmptyStrings(x) FROM file('$input_file', 'TabSeparated', 'x String') FORMAT TabSeparated"
      ;;
    json_remove_duplicate_keys)
      echo "SELECT JSONRemoveDuplicateKeys(x) FROM file('$input_file', 'TabSeparated', 'x String') FORMAT TabSeparated"
      ;;
    json_drop_keys)
      echo "SELECT JSONDropKeys(['a'])(x) FROM file('$input_file', 'TabSeparated', 'x String') FORMAT TabSeparated"
      ;;
    json_clean_posthog_event_properties)
      echo "SELECT JSONCleanPostHogEventProperties(x) FROM file('$input_file', 'TabSeparated', 'x String') FORMAT TabSeparated"
      ;;
    decompress)
      echo "SELECT hex(decompress(unhex(data), codec)) FROM file('$input_file', 'TabSeparated', 'codec String, data String') FORMAT TabSeparated"
      ;;
    *)
      echo "Unknown UDF: $1" >&2
      return 1
      ;;
  esac
}
