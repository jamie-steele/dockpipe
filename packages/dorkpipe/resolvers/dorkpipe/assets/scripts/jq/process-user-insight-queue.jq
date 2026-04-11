# Input via: jq -n --arg now "$NOW" --slurpfile q queue.json --slurpfile i insights.json --slurpfile r rules.json -f process-user-insight-queue.jq
# $q[0] = queue doc, $i[0] = insights doc or null, $r[0] = rules, $now = ISO8601 UTC

def doc_default:
  if . == null then
    {
      schema_version: "1.0",
      kind: "dockpipe_user_insights",
      separation: {
        user_insights: "This file — structured human guidance; not verified facts.",
        repo_facts: ".dorkpipe/self-analysis/ (example layout)",
        system_findings: "bin/.dockpipe/ci-analysis/findings.json"
      },
      insights: []
    }
  else . end;

# Note: inside [...], "select(...) | $c" is required; bare select can collect null in some jq versions.
def first_classifier($text; $rules):
  [ $rules.classifiers[] as $c | select($text | test($c.pattern)) | $c ] | .[0] // null;

def category_from_match($mc; $rules):
  if $mc != null then $mc.category else $rules.default_category end;

def status_for($cat; $mc; $rules):
  if $mc != null and ($mc.force_review == true) then "pending"
  elif ($rules.review_required_categories | index($cat)) != null then "pending"
  elif ($rules.auto_promote_categories | index($cat)) != null then "accepted"
  else "pending"
  end;

def severity_for($text):
  if ($text | test("(?i)\\bcritical\\b|severity\\s*[:=]?\\s*1")) then "critical"
  elif ($text | test("(?i)\\bhigh\\b|severity\\s*[:=]?\\s*2")) then "high"
  elif ($text | test("(?i)\\bmedium\\b|severity\\s*[:=]?\\s*3")) then "medium"
  elif ($text | test("(?i)\\blow\\b|severity\\s*[:=]?\\s*4")) then "low"
  else "unspecified"
  end;

def norm_text($t): $t | gsub("\\s+"; " ") | gsub("^\\s+|\\s+$"; "");

def build_insight($item; $rules; $now):
  ($item.raw_text) as $raw
  | first_classifier($raw; $rules) as $mc
  | category_from_match($mc; $rules) as $cat
  | status_for($cat; $mc; $rules) as $st
  | {
      id: ("insight-" + $item.id),
      queue_item_id: $item.id,
      category: $cat,
      category_hint: $item.category_hint,
      normalized_text: norm_text($raw),
      intent: "user_guidance_signal",
      severity: severity_for($raw),
      scope: ($item.scope // {}),
      provenance: {
        source: $item.source,
        original_text: $raw,
        captured_at_utc: $item.timestamp_utc,
        processed_at_utc: $now,
        classifier: (if $mc != null then {pattern: $mc.pattern, category: $mc.category} else null end)
      },
      confidence: {
        role: "user_signal",
        score: null,
        note: "Not verified by automated scan; not authoritative truth."
      },
      status: $st,
      supersedes: null,
      stale: false,
      history: []
    };

($q[0]) as $queue
| ($i[0] // null) as $rawdoc
| ($r[0]) as $rules
| ($now) as $now_utc
| ($rawdoc | doc_default) as $doc
| ($doc.insights | map(.queue_item_id)) as $seen
| ($queue.items // []) as $items
| ($items
  | map(select(.id as $id | ($seen | index($id)) == null))
  | map(build_insight(.; $rules; $now_utc))) as $new
| $doc
| .insights += $new
| .provenance = ({
    last_processed_at_utc: $now_utc,
    generator: "dockpipe-user-insight-process",
    new_insights_count: ($new | length)
  })
| .
