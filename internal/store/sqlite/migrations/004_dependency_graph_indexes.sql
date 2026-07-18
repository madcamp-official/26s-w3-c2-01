CREATE INDEX idx_dependencies_project_resources
ON dependencies(source_type, source_id, target_type, relation, target_id);

CREATE INDEX idx_dependencies_resource_projects
ON dependencies(target_type, target_id, source_type, relation, source_id);
