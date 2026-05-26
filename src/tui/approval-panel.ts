import {
  Key,
  matchesKey,
  truncateToWidth,
  visibleWidth,
  wrapTextWithAnsi,
} from "@earendil-works/pi-tui";
import type { Component } from "@earendil-works/pi-tui";
import { bold, cyan, gray, green, red, yellow } from "picocolors";
import type { ApprovalRequest, ApprovalResult } from "../approval";

export class ApprovalPanel implements Component {
  private resolved = false;

  constructor(
    private request: ApprovalRequest,
    private onResolve: (result: ApprovalResult) => void,
  ) {}

  render(width: number): string[] {
    if (width < 8) {
      return [truncateToWidth("Approval", width)];
    }

    const contentWidth = Math.max(1, width - 4);
    const lines: string[] = [
      bold(yellow("Approval Required")),
      "",
      `${gray("Action:")} ${this.request.action}`,
    ];

    if (this.request.subject) {
      lines.push(`${gray("Subject:")} ${this.request.subject}`);
    }

    if (this.request.riskLevel) {
      lines.push(`${gray("Risk:")} ${this.request.riskLevel}`);
    }

    if (this.request.policyReason) {
      lines.push(`${gray("Policy:")} ${this.request.policyReason}`);
    }

    const details = Object.entries(this.request.details);
    if (details.length > 0) {
      lines.push("", gray("Details:"));

      for (const [key, value] of details) {
        lines.push(...this.wrapCapped(`${gray(`${key}:`)} ${value}`, contentWidth, 6));
      }
    }

    lines.push("", gray("Options:"));
    lines.push(`${green("y")} approve once`);

    if (this.request.suggestedPolicyAmendment) {
      lines.push(
        `${cyan("a")} always allow prefix: ${this.request.suggestedPolicyAmendment.prefix}`,
      );
    }

    lines.push(`${red("n")} deny`);
    lines.push(gray("Esc or Ctrl+C denies this request."));

    return [
      yellow(`┌${"─".repeat(width - 2)}┐`),
      ...lines.flatMap((line) => this.frameWrappedLine(line, contentWidth)),
      yellow(`└${"─".repeat(width - 2)}┘`),
    ];
  }

  invalidate(): void {
    // Stateless render.
  }

  handleInput(data: string): void {
    if (this.resolved) return;

    if (matchesKey(data, "y") || matchesKey(data, Key.shift("y"))) {
      this.resolve({ decision: "approve_once" });
      return;
    }

    if (
      this.request.suggestedPolicyAmendment &&
      (matchesKey(data, "a") || matchesKey(data, Key.shift("a")))
    ) {
      this.resolve({
        decision: "always_allow_command_prefix",
        policyAmendment: this.request.suggestedPolicyAmendment,
      });
      return;
    }

    if (
      matchesKey(data, "n") ||
      matchesKey(data, Key.shift("n")) ||
      matchesKey(data, Key.escape) ||
      matchesKey(data, Key.ctrl("c"))
    ) {
      this.resolve({
        decision: "deny",
        reason: "Denied in terminal UI.",
      });
    }
  }

  private resolve(result: ApprovalResult): void {
    this.resolved = true;
    this.onResolve(result);
  }

  private frameWrappedLine(line: string, contentWidth: number): string[] {
    return this.wrapLine(line, contentWidth).map((wrapped) => {
      const text = truncateToWidth(wrapped, contentWidth);
      const padding = " ".repeat(Math.max(0, contentWidth - visibleWidth(text)));
      return `${yellow("│")} ${text}${padding} ${yellow("│")}`;
    });
  }

  private wrapCapped(
    text: string,
    contentWidth: number,
    maxLines: number,
  ): string[] {
    const lines = this.wrapLine(text, contentWidth);

    if (lines.length <= maxLines) {
      return lines;
    }

    return [
      ...lines.slice(0, maxLines - 1),
      gray(`... ${lines.length - maxLines + 1} more lines`),
    ];
  }

  private wrapLine(text: string, contentWidth: number): string[] {
    const wrapped = text.split("\n").flatMap((line) =>
      line
        ? wrapTextWithAnsi(line, contentWidth)
        : [""]
    );

    return wrapped.length > 0 ? wrapped : [""];
  }
}
