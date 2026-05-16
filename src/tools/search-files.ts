import { tool } from "ai";
import { execa } from "execa";
import path from "node:path";
import { z } from "zod";
import { resolveExistingProjectPath } from "../project-path.js";

const ignoredGlobs = ["!.git/**", "!node_modules/**", "!dist/**", "!build/**", "!.next/**"];

type SearchMatch = {
  file: string;
  line: number;
  text: string;
};

function parseRipgrepLine(line: string, root: string): SearchMatch | null {
  const firstSeparator = line.indexOf(":");
  const secondSeparator = line.indexOf(":", firstSeparator + 1);

  if (firstSeparator === -1 || secondSeparator === -1) {
    return null;
  }

  const filePath = line.slice(0, firstSeparator);
  const lineNumber = Number(line.slice(firstSeparator + 1, secondSeparator));
  const text = line.slice(secondSeparator + 1);

  if (!Number.isInteger(lineNumber)) {
    return null;
  }

  return {
    file: path.relative(root, filePath),
    line: lineNumber,
    text,
  };
}

export const searchFilesTool = tool({
  description: "Search text in project files",

  inputSchema: z.object({
    query: z.string().min(1),
    path: z.string().default("."),
    maxResults: z.number().int().min(1).max(100).default(20),
    caseSensitive: z.boolean().default(false),
  }),

  execute: async ({ query, path, maxResults, caseSensitive }) => {
    const projectPath = await resolveExistingProjectPath(path);
    const args = [
      "--line-number",
      "--no-heading",
      "--with-filename",
      "--color=never",
      "--fixed-strings",
      ...ignoredGlobs.flatMap((glob) => ["--glob", glob]),
    ];

    if (!caseSensitive) {
      args.push("--ignore-case");
    }

    args.push(query, projectPath.absolutePath);

    const result = await execa("rg", args, {
      reject: false,
    });

    if (result.exitCode === 1) {
      return {
        matches: [],
      };
    }

    if (result.exitCode !== 0) {
      return {
        matches: [],
        error: result.stderr,
        exitCode: result.exitCode,
      };
    }

    const matches = result.stdout
      .split("\n")
      .filter(Boolean)
      .slice(0, maxResults)
      .map((line) => parseRipgrepLine(line, projectPath.root))
      .filter((match): match is SearchMatch => match !== null);

    return {
      matches,
    };
  },
});
