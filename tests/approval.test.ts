import { describe, expect, it } from "vitest";
import { requestApproval, type ApprovalRequest } from "../src/approval.js";

describe("requestApproval", () => {
  it("delegates approval decisions to the configured prompt", async () => {
    const request: ApprovalRequest = {
      action: "run-command",
      title: "Run command",
      details: {
        Command: "pnpm install",
      },
    };

    const approved = await requestApproval(request, async (receivedRequest) => {
      expect(receivedRequest).toEqual(request);
      return true;
    });

    expect(approved).toBe(true);
  });
});
