#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Mapping-driven evidence checks. Helpers return nonzero with an error on stderr;
# they never update the caller's assertion counters or search for fallback files.

EVIDENCE_SCAN_SUBDIR='.complytime/scan'

evidence_error() {
    echo "evidence: $*" >&2
    return 1
}

# Resolve an existing path without allowing traversal or symlinks outside root.
evidence_path() {
    local root="$1" coordinate="$2" resolved
    root=$(realpath -e -- "${root}") || {
        evidence_error "cannot resolve workspace or scan root: ${root}"
        return 1
    }
    [[ "${coordinate}" == /* ]] || coordinate="${root}/${coordinate}"
    resolved=$(realpath -e -- "${coordinate}") || {
        evidence_error "missing path: ${coordinate}"
        return 1
    }
    if [[ "${resolved}" != "${root}/"* ]]; then
        evidence_error "path escapes workspace: ${coordinate}"
        return 1
    fi
    if [[ ! -f "${resolved}" || ! -r "${resolved}" || ! -s "${resolved}" ]]; then
        evidence_error "not a readable nonempty regular file: ${coordinate}"
        return 1
    fi
    printf '%s\n' "${resolved}"
}

# Accept absolute local RFC 3986 file URLs only. Decode percent escapes as bytes,
# not shell syntax; reject control bytes before they can enter shell variables.
evidence_file_url() {
    local url="$1" encoded decoded="" character hex
    [[ "${url}" == file:///* ]] || {
        evidence_error "expected an absolute local file URL"
        return 1
    }
    encoded="${url#file://}"
    while [[ -n "${encoded}" ]]; do
        character="${encoded:0:1}"
        if [[ "${character}" == '%' ]]; then
            hex="${encoded:1:2}"
            if [[ ! "${hex}" =~ ^[[:xdigit:]]{2}$ || "${hex^^}" =~ ^(0[0-9A-F]|1[0-9A-F]|7F)$ ]]; then
                evidence_error "invalid file URL escape"
                return 1
            fi
            printf -v character '%b' "\\x${hex}"
            encoded="${encoded:3}"
        else
            if [[ ! "${character}" =~ ^[a-zA-Z0-9/._~!\$\&\'\(\)\*+,\;=:@-]$ ]]; then
                evidence_error "unescaped or unsupported file URL character"
                return 1
            fi
            encoded="${encoded:1}"
        fi
        decoded+="${character}"
    done
    printf '%s\n' "${decoded}"
}

# Select only the path reported by this invocation for the expected target.
# No directory listing, mtime, or filename ordering participates in selection.
select_evaluation_log() {
    local workspace="$1" output="$2" policy="$3" target="$4"
    local line path="" count=0 resolved
    local report_pattern='^Evaluation log written: (.+) \[target: (.+)\]$'
    while IFS= read -r line; do
        if [[ "${line}" =~ ${report_pattern} && "${BASH_REMATCH[2]}" == "${target}" ]]; then
            path="${BASH_REMATCH[1]}"
            count=$((count + 1))
        fi
    done <<< "${output}"
    [[ "${count}" -eq 1 && "${path}" == *.json ]] || {
        evidence_error "expected exactly one reported JSON log for ${target}"
        return 1
    }
    [[ "${path}" == /* ]] || path="${workspace}/${path}"
    path=$(evidence_path "${workspace}" "${path}") || return 1
    resolved=$(evidence_path "${workspace}/${EVIDENCE_SCAN_SUBDIR}" "${path}") || return 1
    jq -es --arg policy "${policy}" --arg target "${target}" '
        length == 1 and (.[0] | .metadata.id == $policy and .target.id == $target)
    ' "${resolved}" >/dev/null || {
        evidence_error "malformed or mismatched EvaluationLog"
        return 1
    }
    printf '%s\n' "${resolved}"
}

# Return validated evidence records as JSON. Repeated use of an identical
# instance across assessments is allowed; inconsistent instance identities fail.
validate_mapped_evidence() {
    local workspace="$1" log="$2" policy="$3" target="$4" requirement="$5"
    local sources source coordinate digest reference url path url_path actual role records='[]'
    sources=$(jq -ecs --arg policy "${policy}" --arg target "${target}" \
        --arg requirement "${requirement}" '
        def text: type == "string" and length > 0 and
            (test("[\u0000-\u001f\u007f]") | not);
        # Provider IDs distinguish attestation roles. References use the
        # resolved repository identity, not a workspace policy alias.
        def role($prefix): (.id | type == "string") and (.id | startswith($prefix));
        select(length == 1) | .[0] |
        select(.metadata.id == $policy and .target.id == $target) |
        select((.evaluations | type == "array") and
            (.metadata."mapping-references" | type == "array")) |
        [.evaluations[]."assessment-logs"[] |
            select(.requirement."entry-id" == $requirement and
                .requirement."reference-id" == $policy)] |
        select(length > 0) |
        if any(.[]; (.evidence | type) != "array") then error("evidence must be an array")
        else . end |
        select(all(.[]; any(.evidence[]; role("ampel_")) and
            any(.evidence[]; role("snappy_")))) |
        [.[].evidence[] | select(role("ampel_") or role("snappy_"))] |
        select(all(.[]; (.id | text) and (.source."reference-id" | text) and
            (.source.coordinate | text) and (.source.digest | text)))
    ' "${log}") || {
        evidence_error "missing or malformed evidence on ${requirement}"
        return 1
    }
    while IFS= read -r source; do
        coordinate=$(jq -r '.source.coordinate' <<< "${source}")
        digest=$(jq -r '.source.digest' <<< "${source}")
        reference=$(jq -r '.source."reference-id"' <<< "${source}")
        role=$(jq -r '.id | if startswith("ampel_") then "ampel" else "snappy" end' \
            <<< "${source}")
        [[ "${digest}" =~ ^sha256:[[:xdigit:]]{64}$ ]] || {
            evidence_error "unsupported or malformed SHA-256 digest"
            return 1
        }
        path=$(evidence_path "${workspace}" "${coordinate}") || return 1
        actual=$(sha256sum < "${path}") || {
            evidence_error "cannot hash evidence: ${coordinate}"
            return 1
        }
        actual="sha256:${actual:0:64}"
        [[ "${actual}" == "${digest,,}" ]] || {
            evidence_error "digest mismatch: ${coordinate}"
            return 1
        }
        url=$(jq -er --arg reference "${reference}" '
            [.metadata."mapping-references"[] | select(.id == $reference)] |
            select(length == 1) | .[0].url | select(type == "string" and length > 0 and
                (test("[\u0000-\u001f\u007f]") | not))
        ' "${log}") || {
            evidence_error "missing, duplicate, or malformed reference: ${reference}"
            return 1
        }
        url_path=$(evidence_file_url "${url}") || return 1
        url_path=$(evidence_path "${workspace}" "${url_path}") || return 1
        [[ "${path}" == "${url_path}" ]] || {
            evidence_error "coordinate and URL identify different files"
            return 1
        }
        if [[ "${role}" == ampel ]]; then
            jq -e --arg requirement "${requirement}" \
                'any(.predicate.results[]; .policy.id == $requirement)' "${path}" \
                >/dev/null || {
                evidence_error "Ampel attestation lacks ${requirement}"
                return 1
            }
        fi
        records=$(jq -c --argjson source "${source}" --arg path "${path}" \
            --arg role "${role}" '. + [{id: $source.id,
                reference: $source.source."reference-id", coordinate: $source.source.coordinate,
                path: $path, digest: $source.source.digest, role: $role}]' <<< "${records}")
    done < <(jq -c '.[]' <<< "${sources}")
    jq -ec 'unique | . as $records |
        select(all(["id", "reference", "coordinate", "path"][];
            . as $key | ($records | unique_by(.[$key]) | length) == ($records | length)))
    ' <<< "${records}" || {
        evidence_error "distinct evidence instances reuse an identity or coordinate"
        return 1
    }
}

# Revalidate both snapshots after evaluation B; require independently addressable
# instances even if the two evaluations returned identical artifact bytes.
validate_evidence_history() {
    local workspace="$1" first="$2" second="$3" policy="$4" target="$5" requirement="$6"
    local earlier later
    earlier=$(validate_mapped_evidence "${workspace}" "${first}" "${policy}" \
        "${target}" "${requirement}") || return 1
    later=$(validate_mapped_evidence "${workspace}" "${second}" "${policy}" \
        "${target}" "${requirement}") || return 1
    jq -en --argjson earlier "${earlier}" --argjson later "${later}" '
        all(["id", "reference", "coordinate", "path"][]; . as $key |
            all($earlier[]; .[$key] as $value | all($later[]; .[$key] != $value)))
    ' >/dev/null || {
        evidence_error "evaluations collapsed distinct artifact instances"
        return 1
    }
}
