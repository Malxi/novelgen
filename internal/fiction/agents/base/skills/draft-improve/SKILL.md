# Draft Improve Skill

## Purpose
Improve draft chapters based on review suggestions and feedback.

## Input
- `story_setup`: The story setup containing premise, genres, themes, tense, POV style, etc.
- `chapter`: Chapter information including ID, title, summary, beats, etc.
- `state_matrix`: Current story state formatted as string
- `target_words`: Target word count for the chapter
- `current_draft`: The current draft content that needs improvement
- `suggestions`: Review suggestions for improvement
- `context`: Optional continuity context from previous chapters
- `recap`: Optional canonical recap from previous chapter
- `next_chapters`: Optional array of upcoming chapter info for foreshadowing
- `custom_prompt`: Optional additional instructions

## Output
- `content`: The improved draft chapter content as a string

## Guidelines

1. **Address all suggestions** - Fix the issues identified in the review
2. **Preserve good content** - Keep what works, only change what needs fixing
3. **Maintain continuity** - Ensure changes don't break continuity with previous/next chapters
4. **Keep chapter structure** - Maintain the overall flow and pacing
5. **Match target length** - Aim for the specified word count
6. **Follow style guidelines** - Maintain tense, POV, genre, and tone consistency

## Improvement Priorities

1. **Critical Issues** (High Priority):
   - Plot holes or inconsistencies
   - Character behavior that contradicts established traits
   - Broken continuity with previous chapters
   - Missing required beats or events

2. **Quality Issues** (Medium Priority):
   - Weak dialogue or character voices
   - Pacing problems
   - Insufficient sensory details
   - Unclear scene transitions

3. **Polish Issues** (Low Priority):
   - Word choice improvements
   - Sentence flow
   - Minor description enhancements

## Output Format

Return the complete improved chapter as a single text string. Do not include:
- Explanations of changes made
- Lists of what was fixed
- Meta-commentary about the writing process
