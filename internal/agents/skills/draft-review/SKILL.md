# Draft Review Skill

## Purpose
Review draft chapters and provide detailed feedback for improvement.

## Input
- `story_setup`: The story setup for context
- `chapter`: Chapter information including ID, title, summary, beats, target word count
- `draft_content`: The draft content to review
- `target_words`: Target word count for the chapter
- `iteration`: Current iteration number

## Output
- `review_result`: Review result containing scores and suggestions

## Review Result Structure

The review result must include:
- **overall_score**: Overall quality score (0-100)
- **dimensions**: Array of dimension scores:
  - `continuity`: Continuity with previous chapters (0-25)
  - `plot_coherence`: Plot coherence and beat coverage (0-25)
  - `character_consistency`: Character consistency and voice (0-25)
  - `writing_quality`: Writing quality and style (0-25)
- **summary**: Brief summary of the review
- **strengths**: Array of strong points
- **weaknesses**: Array of weak points
- **suggestions`: Array of specific improvement suggestions with:
  - `target_id`: "draft" or specific section
  - `issue`: Description of the problem
  - `suggestion`: How to fix it
  - `priority`: "high", "medium", or "low"

## Review Dimensions

### Continuity (0-25 points)
- Smooth transition from previous chapter
- Consistent with established story state
- Proper handling of recap/next chapter hints
- Scene-anchor rules followed

### Plot Coherence (0-25 points)
- All chapter beats are covered
- Events flow logically
- Pacing matches chapter requirements
- Opening and closing beats handled correctly

### Character Consistency (0-25 points)
- Characters act according to their established traits
- Dialogue matches character voices
- Motivations and goals are consistent
- Character presence matches outline

### Writing Quality (0-25 points)
- Show, don't tell
- Good balance of dialogue, action, description
- Sensory details present
- Appropriate style for genre

## Guidelines

1. Be specific in suggestions - cite examples where possible
2. Prioritize high-impact issues
3. Acknowledge strengths as well as weaknesses
4. Provide actionable feedback that can be implemented
5. Consider the iteration number - be more lenient on early iterations
