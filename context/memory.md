# Memory

- For background tasks (server execution, etc.), check status immediately and report to prevent timeout
  - Server keeps running, so waiting for output causes timeout
  - After execution, check port with netstat and report results immediately
- **Absolutely forbidden: Do NOT use `taskkill //F //IM node.exe`!**
  - Instead, kill by port: `npx kill-port 5173 5174`
  - Or kill specific process by PID
- Problem-solving approach for mobile coders
  - Hard to see code on mobile → Error logs and explanations serve as "eyes"
  - When problem occurs: Check logs → Explain what the problem is → Wait for user judgment → Fix
  - Before modifying code, explain "why this problem occurred" first
  - If user says "explain", "analyze", "why?" → Only explain, don't modify
  - Through this process, user can grow to give more accurate instructions
- When user asks a question, answer the question first; when they request work, do the work
