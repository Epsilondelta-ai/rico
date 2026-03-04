# Q&A Auto Recording

- When user asks about concepts/architecture/technology, and after explanation the user seems to understand, automatically record to Q&A
- Judgment criteria: Positive reactions like "I see", "Got it", "Good", "Right"
- Recording location: `qna/YYYY-MM-DD.md` (today's date)
- Recording format:

  ```
  ## [Topic]

  **Q: [Question summary]**

  [Explanation content]
  ```

- If file for that date doesn't exist, create new; if exists, append to bottom
- **Important:** Before recording, always ask user "Should I record this to Q&A?", and record only if user agrees
