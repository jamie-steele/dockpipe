def gosec_issues($g):
  ($g.Issues // []) | map(
    . as $i
    | {
        tool: "gosec",
        rule_id: ($i.rule_id // "unknown"),
        title: ($i.details // ""),
        file: ($i.file // ""),
        line: (($i.line // "0") | tonumber? // 0),
        column: (($i.column // "0") | tonumber? // 0),
        severity: ($i.severity // ""),
        confidence: ($i.confidence // ""),
        category: (if ($i.cwe != null) and ($i.cwe.id != null) then "CWE-\($i.cwe.id)" else "sast" end),
        message: ($i.details // ""),
        remediation: null,
        raw: $i
      }
  );

def govulncheck_vulns($v):
  ($v | .vulns // .Vulns // []) | map(
    . as $vn
    | ($vn.osv // $vn.OSV // {}) as $osv
    | {
        tool: "govulncheck",
        rule_id: ($osv.id // "unknown"),
        title: ($osv.summary // $osv.id // "vulnerability"),
        file: "",
        line: 0,
        column: 0,
        severity: (
          if ($osv.severity != null) and ($osv.severity | type == "array") and ($osv.severity | length > 0)
          then (($osv.severity[0] | .type // .score // "unknown") | tostring)
          else "unknown" end
        ),
        confidence: "",
        category: "dependency-vuln",
        message: (($osv.details // $osv.summary // "") | tostring | if length > 2000 then .[0:2000] else . end),
        remediation: "Upgrade affected module(s); inspect raw.govulncheck and module traces.",
        raw: $vn
      }
  );

gosec_issues($gosec[0] // {}) + govulncheck_vulns($gv[0] // {})
