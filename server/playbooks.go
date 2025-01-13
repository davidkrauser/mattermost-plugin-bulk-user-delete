package main

import (
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

func purgeDanglingPlaybookMembers(db *sql.DB) error {
	if _, err := db.Exec(`
			DELETE FROM ir_category
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_category.userid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_playbookautofollow
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_playbookautofollow.userid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_playbookmember
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_playbookmember.memberid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_run_participants
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_run_participants.userid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_viewedchannel
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_viewedchannel.userid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_userinfo
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = ir_userinfo.id
			  );
		`); err != nil {
		return err
	}

	return nil
}

func purgeEmptyPlaybooks(db *sql.DB) error {
	rows, err := db.Query(`
			SELECT id FROM ir_playbook
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM ir_playbookmember
			      WHERE ir_playbookmember.playbookid = ir_playbook.id
			  );
		`)
	if err != nil {
		return fmt.Errorf("error finding playbooks with no members: %s", err.Error())
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return fmt.Errorf("error parsing board IDs: %s", scanErr.Error())
		}
		ids = append(ids, id)
	}

	playbookDeleteQuery := sq.Delete("ir_playbook").
		Where(sq.Eq{"id": ids}).
		PlaceholderFormat(sq.Dollar)

	playbookDeleteQueryString, playbookDeleteQueryArgs, err := playbookDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbooks: %s", err.Error())
	}

	_, err = db.Exec(playbookDeleteQueryString, playbookDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbooks: %s", err.Error())
	}

	return nil
}

func purgeEmptyPlaybookRuns(db *sql.DB) error {
	rows, err := db.Query(`
			SELECT id FROM ir_incident
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM ir_run_participants
			      WHERE ir_run_participants.incidentid = ir_incident.id
			  );
		`)
	if err != nil {
		return fmt.Errorf("error finding playbook runs with no members: %s", err.Error())
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return fmt.Errorf("error parsing board IDs: %s", scanErr.Error())
		}
		ids = append(ids, id)
	}

	runDeleteQuery := sq.Delete("ir_incident").
		Where(sq.Eq{"id": ids}).
		PlaceholderFormat(sq.Dollar)

	runDeleteQueryString, runDeleteQueryArgs, err := runDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete playbook runs: %s", err.Error())
	}

	_, err = db.Exec(runDeleteQueryString, runDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete playbook runs: %s", err.Error())
	}

	return nil
}

func purgeDanglingPlaybookData(db *sql.DB) error {
	if _, err := db.Exec(`
			DELETE FROM ir_metricconfig
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM ir_playbook
			      WHERE ir_playbook.id = ir_metricconfig.playbookid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_metric
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM ir_incident
			      WHERE ir_incident.id = ir_metric.incidentid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_statusposts
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM ir_incident
			      WHERE ir_incident.id = ir_statusposts.incidentid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_timelineevent
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM ir_incident
			      WHERE ir_incident.id = ir_timelineevent.incidentid
			  );
		`); err != nil {
		return err
	}

	if _, err := db.Exec(`
			DELETE FROM ir_channelaction
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Channels
			      WHERE Channels.id = ir_channelaction.channelid
			  );
		`); err != nil {
		return err
	}

	return nil
}
