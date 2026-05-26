import { describe, expect, it } from "bun:test";
import { parseSgrMouseWheel } from "../src/tui/mouse";

describe("parseSgrMouseWheel", () => {
  it("parses SGR wheel up and down events", () => {
    expect(parseSgrMouseWheel("\x1b[<64;10;20M")).toBe("up");
    expect(parseSgrMouseWheel("\x1b[<65;10;20M")).toBe("down");
  });

  it("parses modified wheel events by ignoring modifier bits", () => {
    expect(parseSgrMouseWheel("\x1b[<68;10;20M")).toBe("up");
    expect(parseSgrMouseWheel("\x1b[<69;10;20M")).toBe("down");
  });

  it("ignores non-wheel mouse and non-mouse input", () => {
    expect(parseSgrMouseWheel("\x1b[<0;10;20M")).toBeUndefined();
    expect(parseSgrMouseWheel("\x1b[A")).toBeUndefined();
  });
});
