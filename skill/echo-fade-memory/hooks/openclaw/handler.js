/**
 * Echo Fade Memory hook for OpenClaw.
 *
 * Injects a bootstrap reminder so the agent uses the local memory service
 * as an active part of its workflow.
 */

const REMINDER_CONTENT = `
## Echo Fade Memory Reminder

Before answering:
- Recall relevant project memory first
- Store durable user preferences, decisions, and corrections
- Reinforce memories that proved useful
- Ground memories if they look fuzzy or uncertain

Use the local service workflow from the echo-fade-memory skill package.
`.trim();

const handler = async (event) => {
  if (!event || typeof event !== "object") {
    return;
  }

  if (event.type !== "agent" || event.action !== "bootstrap") {
    return;
  }

  if (!event.context || typeof event.context !== "object") {
    return;
  }

  if (Array.isArray(event.context.bootstrapFiles)) {
    event.context.bootstrapFiles.push({
      path: "ECHO_FADE_MEMORY_REMINDER.md",
      content: REMINDER_CONTENT,
      virtual: true,
    });
  }
};

module.exports = handler;
module.exports.default = handler;
