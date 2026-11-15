// terraform plan の結果を PR にコメントする (ci-infra の actions/github-script から呼ぶ)
//   - 同じ PR へのコメントは 1 つにまとめ、push のたびに書き換える (目印のコメントで探す)
//   - コメントの上限 (65536 文字) を超えないよう、plan が長いときは末尾だけを残す
//
// 環境変数
//   - PLAN_FILE: plan の出力を書いたファイルのパス
//   - PLAN_EXITCODE: terraform plan の終了コード
//   - ENV_NAME: 対象の環境 (prd など)

const fs = require("fs");

const MARKER = "<!-- ci-infra-plan -->";
const LIMIT = 60000;

module.exports = async ({ github, context }) => {
  const planFile = process.env.PLAN_FILE;
  if (!planFile) {
    throw new Error("PLAN_FILE is not set");
  }
  const raw = fs.readFileSync(planFile, "utf8");
  const plan = raw.length > LIMIT ? raw.slice(-LIMIT) : raw;

  const status = process.env.PLAN_EXITCODE === "0" ? "Success" : "Failed";
  const body = [
    MARKER,
    `### terraform plan (${process.env.ENV_NAME}): ${status}`,
    "",
    "<details><summary>Result</summary>",
    "",
    "```",
    plan,
    "```",
    "",
    "</details>",
  ].join("\n");

  const { owner, repo } = context.repo;
  const issue_number = context.issue.number;
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner,
    repo,
    issue_number,
  });
  const existing = comments.find((c) => c.body && c.body.startsWith(MARKER));

  if (!existing) {
    await github.rest.issues.createComment({ owner, repo, issue_number, body });
    return;
  }

  await github.rest.issues.updateComment({
    owner,
    repo,
    comment_id: existing.id,
    body,
  });
};
