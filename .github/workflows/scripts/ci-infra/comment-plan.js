// terraform plan の結果を PR にコメントする (ci-infra の actions/github-script から呼ぶ)
//   - 同じ PR へのコメントは 1 つにまとめ、push のたびに書き換える (目印のコメントで探す)
//   - <details> の外に、何回目の plan か、対象のコミット、plan を実行した日時、plan の要約 (Plan: ... / No changes.) を書く
//     GitHub の編集履歴の画面は <details> を畳んだまま差分を出すため、開かなくても版ごとの違いが分かるようにする
//   - 何回目かは、本文に残した HTML コメント (目に見えない) の数を、書き換えるたびに 1 つ増やして数える
//   - コメントの上限 (65536 文字) を超えないよう、plan が長いときは末尾だけを残す
//
// 環境変数
//   - PLAN_FILE: plan の出力を書いたファイルのパス
//   - PLAN_EXITCODE: terraform plan の終了コード
//   - ENV_NAME: 対象の環境 (prd など)

const fs = require("fs");

const MARKER = "<!-- ci-infra-plan -->";
const COUNT_PATTERN = /<!-- ci-infra-plan-count: (\d+) -->/;
const LIMIT = 60000;

// plan の出力から、要約の 1 行を取り出す
//   - 差分があるとき: "Plan: 1 to add, 0 to change, 0 to destroy."
//   - 差分がないとき: "No changes."
//   - どちらもないとき (エラーや、出力値だけの変更): 詳細を見るよう促す
const summarize = (plan) => {
  const planLine = plan.match(/^Plan: .*$/m);
  if (planLine) {
    return planLine[0];
  }
  if (/^No changes\./m.test(plan)) {
    return "No changes.";
  }
  return "Plan: (see Result)";
};

// 日本時間で "2026-11-22 11:57:33" の形にする
const formatJst = (date) =>
  date.toLocaleString("sv-SE", { timeZone: "Asia/Tokyo" });

module.exports = async ({ github, context }) => {
  const planFile = process.env.PLAN_FILE;
  if (!planFile) {
    throw new Error("PLAN_FILE is not set");
  }
  const raw = fs.readFileSync(planFile, "utf8");
  const plan = raw.length > LIMIT ? raw.slice(-LIMIT) : raw;

  const { owner, repo } = context.repo;
  const issue_number = context.issue.number;
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner,
    repo,
    issue_number,
  });
  const existing = comments.find((c) => c.body && c.body.startsWith(MARKER));

  const previousCount = existing
    ? Number(existing.body.match(COUNT_PATTERN)?.[1] ?? 0)
    : 0;
  const count = previousCount + 1;

  // pull_request のイベントでは、context.sha は PR をマージした仮のコミットになるため、PR の先頭のコミットを使う
  const sha = context.payload.pull_request?.head?.sha ?? context.sha;

  const status = process.env.PLAN_EXITCODE === "0" ? "Success" : "Failed";
  const body = [
    MARKER,
    `<!-- ci-infra-plan-count: ${count} -->`,
    `### terraform plan (${process.env.ENV_NAME}): ${status}`,
    "",
    `- Count: ${count}`,
    `- Commit: ${sha.slice(0, 7)}`,
    `- Planned at: ${formatJst(new Date())} JST`,
    `- ${summarize(plan)}`,
    "",
    "<details><summary>Result</summary>",
    "",
    "```",
    plan,
    "```",
    "",
    "</details>",
  ].join("\n");

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
