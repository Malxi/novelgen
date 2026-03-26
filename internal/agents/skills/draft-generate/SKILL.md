# Draft Generate Skill

## Purpose
Generate draft chapters based on story state, outline, and continuity context.

## Input
- `story_setup`: The story setup containing premise, genres, themes, tense, POV style, etc.
- `chapter`: Chapter information including ID, title, summary, beats, opening/closing beats, pacing, characters, location
- `state_matrix`: Current story state formatted as string (character states, relationships, storylines)
- `context`: Optional continuity context from previous chapters
- `recap`: Optional canonical recap from previous chapter for continuity
- `next_chapters`: Optional array of upcoming chapter info for foreshadowing
- `custom_prompt`: Optional additional instructions

## Output
- `content`: The generated draft chapter content as a string

## Guidelines

1. **Write in the specified language** - All output must be in the language specified in story_setup
2. **Follow narrative tense** - Use the tense from story setup (past/present)
3. **Maintain POV consistency** - Use the POV style from story setup (first/third person, limited/omniscient)
4. **Match genre and style** - Adapt writing style to fit the story's genre and tone
5. **Cover all chapter beats** - Ensure each beat from the chapter outline is addressed
6. **Use state matrix** - Reference character motivations, goals, and emotional states
7. **Maintain continuity** - Use context and recap to ensure smooth transitions from previous chapters
8. **Foreshadow upcoming chapters** - Subtly hint at future events when next_chapters is provided

## Chapter Structure

1. **Continuation Bridge** (if not first chapter): Directly continue from previous chapter's final moment
2. **Opening**: Establish scene and characters present
3. **Rising Action**: Develop events and character interactions
4. **Climax**: The turning point or key moment
5. **Resolution**: Wrap up immediate conflicts while setting up future ones
6. **Closing Hook**: End with something that makes readers want to continue

## Important Rules

- DO NOT include summaries or meta descriptions
- DO NOT write "本章讲述了..." or similar
- Start directly with story content
- Show, don't tell - use sensory details and actions
- Include dialogue that reveals character and advances plot
- Balance description, action, and dialogue
