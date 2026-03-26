# Write Review Skill

## Purpose
Review final chapter content with publication-quality standards.

## Input
- `story_setup`: The story setup for context
- `chapter`: Chapter information
- `chapter_content`: The final chapter content to review
- `target_words`: Target word count
- `iteration`: Current iteration number

## Output
- `review_result`: Review result with scores and suggestions

## Review Result Structure

- **overall_score**: Overall quality score (0-100)
- **dimensions**: Array of dimension scores:
  - `continuity`: Continuity with previous chapters (0-20)
  - `plot_execution`: Plot execution and beat coverage (0-20)
  - `characterization`: Character portrayal and voice (0-20)
  - `prose_quality`: Prose quality and style (0-20)
  - `emotional_impact`: Emotional impact and engagement (0-20)
- **summary**: Brief summary
- **strengths**: Array of strong points
- **weaknesses**: Array of weak points
- **suggestions**: Array of improvement suggestions

## Review Standards

### Publication Quality Criteria

1. **Professional Prose**
   - No awkward phrasing
   - Smooth transitions
   - Appropriate vocabulary
   - Engaging voice

2. **Strong Characterization**
   - Believable characters
   - Distinct voices
   - Consistent behavior
   - Emotional authenticity

3. **Compelling Plot**
   - Clear progression
   - Logical causality
   - Proper pacing
   - Satisfying structure

4. **Immersive World**
   - Vivid details
   - Consistent setting
   - Atmospheric writing
   - Sensory engagement

5. **Reader Engagement**
   - Hook maintains interest
   - Tension builds appropriately
   - Emotional investment
   - Desire to continue reading

## Scoring Guide

- **90-100**: Exceptional, ready for publication
- **80-89**: Very good, minor polish needed
- **70-79**: Good, some improvements needed
- **60-69**: Acceptable, significant improvements needed
- **Below 60**: Needs major revision
