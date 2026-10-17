# Craft Organizations Skill

## Purpose
Generate detailed organization and faction profiles based on story setup and outline context.

## Input
- `story_setup`: Story setup, including world systems, resources, and storylines
- `outline`: Complete story outline structure
- `relevant_chapters`: Chapter summaries where these organizations appear or exert pressure
- `organizations`: List of organization names to generate
- `custom_prompt`: Optional additional instructions

## Output
- `organizations`: Map of organization names to detailed organization profiles

## Organization Profile Structure

Each organization must include:
- **name**: Full organization name
- **type**: Organization type, such as sect, guild, company, empire, army, cult, clan, agency, alliance, hive, or faction
- **description**: Overall description
- **founding**: Origin or founding background (optional)
- **headquarters**: Primary base or seat of power (optional)
- **leadership**: Current leadership or command structure (optional)
- **members**: Important members or member classes (optional)
- **goals**: Array of concrete goals
- **ideology**: Beliefs, doctrine, or operational logic (optional)
- **resources**: Array of resources, capabilities, assets, or leverage (optional)
- **allies**: Array of allied organizations (optional)
- **enemies**: Array of enemy organizations (optional)
- **reputation**: How others perceive this group (optional)
- **structure**: Internal hierarchy or cells/divisions (optional)
- **significance**: Why this organization matters to the story
- **secrets**: Hidden truths or classified plans (optional)
- **notes**: Additional notes for writers (optional)
- **dsl_tags**: Optional stable tags for RPG-DSL conversion
- **state_effects**: Optional array of static state effects with `target`, `kind`, `field`, `from`, `to`, `delta`, `unit`, `cost`, `note`

## Guidelines

1. **Generate EXACTLY the requested organizations** - no more, no less
2. Treat organizations as durable world actors, not temporary scene labels
3. Use enemy factions, storyline pressure, resources, and relevant chapters to infer goals and methods
4. Keep relationships with characters in the character profiles; here, describe institutional stance and structure
5. Do not invent future victories, defeats, betrayals, or relationship changes beyond the outline
6. Use setup premises and world resources as authoritative systems; do not redefine ability systems here
7. Keep `state_effects` static and conservative, such as default hostility, access rules, or resource control
8. Keep all content in the specified language

## Output Format Example

```json
{
  "Organization Name": {
    "name": "Organization Name",
    "type": "sect",
    "description": "A disciplined frontier sect that controls the northern pass...",
    "founding": "Founded after the first gate breach",
    "headquarters": "Blackstone Citadel",
    "leadership": "A council of three elders",
    "members": ["outer disciples", "gate wardens"],
    "goals": ["control access to the pass", "monopolize spirit ore"],
    "ideology": "Order is survival; freedom without discipline invites disaster",
    "resources": ["trained wardens", "ore mines", "fortified passes"],
    "allies": ["Merchant League"],
    "enemies": ["Red Banner Raiders"],
    "reputation": "Respected but feared",
    "structure": "Outer disciples, inner wardens, elder council",
    "significance": "Controls the route the protagonist must cross",
    "secrets": "The council knows why the old pass was sealed",
    "dsl_tags": ["faction", "gatekeeper"],
    "state_effects": [
      {"target": "protagonist", "kind": "relationship", "field": "standing", "to": "watched", "note": "Default institutional stance when introduced"}
    ],
    "notes": "Use formal titles and ritualized discipline in scenes"
  }
}
```
